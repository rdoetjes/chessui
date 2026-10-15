package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	HostColor string
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
	// Default values
	config = Config{StockfishPath: "stockfish", Threads: 4, Depth: 20, Port: 8081}

	// Try primary path
	paths := []string{"/usr/local/etc/chess-server/config.json", "config.json"}
	for _, path := range paths {
		file, err := os.Open(path)
		if err == nil {
			log.Printf("SERVER: Loading config from %s\n", path)
			err = json.NewDecoder(file).Decode(&config)
			file.Close()
			if err == nil {
				return
			}
			log.Printf("ERROR: Failed to decode config at %s: %v\n", path, err)
		}
	}
	log.Printf("SERVER: Using default configuration (Depth: %d, Port: %d)\n", config.Depth, config.Port)
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
		path, err := exec.LookPath(config.StockfishPath)
		if err != nil {
			log.Printf("ERROR: Stockfish not found: %v\n", err)
			return
		}
		cmd := exec.Command(path)
		stdin, _ := cmd.StdinPipe()
		stdout, _ := cmd.StdoutPipe()
		if err := cmd.Start(); err != nil {
			log.Printf("ERROR: Failed to start Stockfish: %v\n", err)
			return
		}
		log.Printf("SERVER: Started Stockfish for room %s\n", roomID)
		room = &Room{ID: roomID, Players: []*websocket.Conn{}, Stockfish: cmd, SFStdin: stdin, SFStdout: stdout}

		// Send engine configuration to the first player
		configMsg, _ := json.Marshal(map[string]any{"type": "config", "depth": config.Depth, "threads": config.Threads})
		conn.WriteMessage(websocket.TextMessage, configMsg)

		go func(rm *Room) {
			// Tell the client that Stockfish is starting
			rm.Mutex.Lock()
			for _, p := range rm.Players {
				p.WriteMessage(websocket.TextMessage, []byte("info string Stockfish Engine Started"))
			}
			rm.Mutex.Unlock()

			scanner := bufio.NewScanner(rm.SFStdout)
			// Increase buffer size to handle long UCI info lines
			const maxCapacity = 1 * 1024 * 1024 // 1MB
			buf := make([]byte, maxCapacity)
			scanner.Buffer(buf, maxCapacity)

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

	// Send engine configuration to every joining player
	configMsg, _ := json.Marshal(map[string]any{"type": "config", "depth": config.Depth, "threads": config.Threads})
	conn.WriteMessage(websocket.TextMessage, configMsg)
	log.Printf("SERVER: Sent config to player in room %s (Depth: %d, Threads: %d)\n", roomID, config.Depth, config.Threads)

	room.Mutex.Unlock()
	roomsMu.Unlock()

	writer := bufio.NewWriter(room.SFStdin)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		msgStr := string(message)

		if strings.HasPrefix(msgStr, "move ") {
			move := strings.TrimPrefix(msgStr, "move ")
			room.Mutex.Lock()
			for _, p := range room.Players {
				if p != conn {
					p.WriteMessage(websocket.TextMessage, []byte("opponent_move "+move))
				}
			}
			room.Mutex.Unlock()
		} else if strings.HasPrefix(msgStr, "player_color ") {
			room.Mutex.Lock()
			room.HostColor = strings.TrimPrefix(msgStr, "player_color ")
			room.Mutex.Unlock()
		} else if msgStr == "request_color" {
			room.Mutex.Lock()
			if room.HostColor != "" {
				conn.WriteMessage(websocket.TextMessage, []byte("assign_color "+room.HostColor))
			}
			room.Mutex.Unlock()
		} else {
			// UCI command -> Flush to Stockfish
			// We don't hold the room lock here to avoid deadlocks with the SFStdout reader
			room.Mutex.Lock()
			w := writer
			room.Mutex.Unlock()

			w.WriteString(msgStr + "\n")
			w.Flush()
		}
	}

	room.Mutex.Lock()
	for i, p := range room.Players {
		if p == conn {
			room.Players = append(room.Players[:i], room.Players[i+1:]...)
			break
		}
	}
	if len(room.Players) == 0 {
		room.Stockfish.Process.Kill()
		delete(rooms, roomID)
	}
	room.Mutex.Unlock()
}

func main() {
	webDir := flag.String("web", "web", "Directory for web assets")
	flag.Parse()

	loadConfig()

	absWebDir, _ := filepath.Abs(*webDir)
	assetsDir := filepath.Join(filepath.Dir(absWebDir), "assets")

	mux := http.NewServeMux()
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("SERVER: Config requested by %s\n", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)
	})
	mux.HandleFunc("/ws", handleWS)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(absWebDir, "index.html"))
			return
		}
		http.FileServer(http.Dir(absWebDir)).ServeHTTP(w, r)
	})
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsDir))))

	log.Printf("SERVER: Listening on http://localhost:%d\n", config.Port)
	log.Printf("SERVER: Serving web assets from %s\n", absWebDir)
	http.ListenAndServe(fmt.Sprintf(":%d", config.Port), mux)
}
