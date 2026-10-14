package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type Config struct {
	StockfishPath string `json:"stockfish_path"`
	Threads       int    `json:"threads"`
	Depth         int    `json:"depth"`
	Port          int    `json:"port"`
}

type Room struct {
	ID        string
	Players   []*websocket.Conn
	Stockfish *exec.Cmd
	SFStdin   io.WriteCloser
	SFStdout  io.ReadCloser
	Mutex     sync.Mutex
}

var (
	config   Config
	rooms    = make(map[string]*Room)
	roomsMu  sync.Mutex
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func loadConfig() {
	wd, _ := os.Getwd()
	log.Printf("SERVER: Working directory: %s\n", wd)

	file, err := os.Open("config.json")
	if err != nil {
		log.Printf("SERVER: Using defaults\n")
		config = Config{StockfishPath: "stockfish", Threads: 2, Depth: 15, Port: 8081}
		return
	}
	defer file.Close()
	json.NewDecoder(file).Decode(&config)
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "lobby"
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	roomsMu.Lock()
	room, exists := rooms[roomID]
	if !exists {
		room = &Room{ID: roomID, Players: []*websocket.Conn{}}
		path, _ := exec.LookPath(config.StockfishPath)
		cmd := exec.Command(path)
		stdin, _ := cmd.StdinPipe()
		stdout, _ := cmd.StdoutPipe()
		cmd.Start()
		room.Stockfish, room.SFStdin, room.SFStdout = cmd, stdin, stdout

		go func(rm *Room) {
			scanner := bufio.NewScanner(rm.SFStdout)
			for scanner.Scan() {
				line := scanner.Text()
				rm.Mutex.Lock()
				for _, p := range rm.Players {
					p.WriteMessage(websocket.TextMessage, []byte(line))
				}
				rm.Mutex.Unlock()
			}
		}(room)
		rooms[roomID] = room
	}
	room.Mutex.Lock()
	room.Players = append(room.Players, conn)
	room.Mutex.Unlock()
	roomsMu.Unlock()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		msgStr := string(message)
		room.Mutex.Lock()
		fmt.Fprintf(room.SFStdin, "%s\n", msgStr)
		if strings.HasPrefix(msgStr, "move ") {
			move := strings.TrimPrefix(msgStr, "move ")
			for _, p := range room.Players {
				if p != conn {
					p.WriteMessage(websocket.TextMessage, []byte("opponent_move "+move))
				}
			}
		}
		room.Mutex.Unlock()
	}

	room.Mutex.Lock()
	for i, p := range room.Players {
		if p == conn {
			room.Players = append(room.Players[:i], room.Players[i+1:]...)
			break
		}
	}
	room.Mutex.Unlock()
}

func main() {
	loadConfig()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "web/index.html")
			return
		}
		http.FileServer(http.Dir("web")).ServeHTTP(w, r)
	})
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	mux.HandleFunc("/ws", handleWS)
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(config)
	})
	log.Printf("SERVER: Listening on http://localhost:%d\n", config.Port)
	http.ListenAndServe(fmt.Sprintf(":%d", config.Port), mux)
}
