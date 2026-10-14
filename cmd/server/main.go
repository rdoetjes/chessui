package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
)

type Config struct {
	StockfishPath string `json:"stockfish_path"`
	Threads       int    `json:"threads"`
	Depth         int    `json:"depth"`
	Port          int    `json:"port"`
}

var (
	config   Config
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func loadConfig() {
	wd, _ := os.Getwd()
	log.Printf("SERVER: Working directory: %s\n", wd)

	file, err := os.Open("config.json")
	if err != nil {
		log.Printf("SERVER: Error opening config.json: %v (Using defaults)\n", err)
		config = Config{StockfishPath: "stockfish", Threads: 2, Depth: 15, Port: 8080}
		return
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		log.Printf("SERVER: Error decoding config.json: %v\n", err)
		return
	}
	log.Printf("SERVER: Config loaded: %+v\n", config)
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	log.Printf("WS: Connection request from %s\n", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS: Upgrade failed: %v\n", err)
		return
	}
	log.Printf("WS: Connection established with %s\n", r.RemoteAddr)
	defer conn.Close()

	log.Printf("WS: Searching for Stockfish: %s\n", config.StockfishPath)
	path, err := exec.LookPath(config.StockfishPath)
	if err != nil {
		log.Printf("WS: Stockfish NOT FOUND: %v\n", err)
		conn.WriteMessage(websocket.TextMessage, []byte("error: stockfish not found"))
		return
	}
	log.Printf("WS: Stockfish found at: %s\n", path)

	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("WS: StdinPipe error: %v\n", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("WS: StdoutPipe error: %v\n", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("WS: Process start error: %v\n", err)
		return
	}
	log.Printf("WS: Stockfish process started (PID: %d)\n", cmd.Process.Pid)
	defer func() {
		log.Printf("WS: Killing Stockfish process %d\n", cmd.Process.Pid)
		cmd.Process.Kill()
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	// Engine -> Client
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			// log.Printf("ENGINE -> WS: %s\n", line)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				return
			}
		}
	}()

	// Client -> Engine
	go func() {
		defer wg.Done()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("WS: Read error: %v\n", err)
				return
			}
			log.Printf("WS -> ENGINE: %s\n", string(message))
			fmt.Fprintf(stdin, "%s\n", string(message))
		}
	}()

	wg.Wait()
	log.Printf("WS: Connection closed for %s\n", r.RemoteAddr)
}

func main() {
	loadConfig()

	mux := http.NewServeMux()

	loggingMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HTTP REQ: %s %s %s\n", r.RemoteAddr, r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})

	// Explicitly serve index.html for the root
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Printf("SERVING: index.html to %s\n", r.RemoteAddr)
			http.ServeFile(w, r, "web/index.html")
			return
		}
		http.FileServer(http.Dir("web")).ServeHTTP(w, r)
	})

	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	mux.HandleFunc("/ws", handleWS)
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("CONFIG REQ from %s\n", r.RemoteAddr)
		json.NewEncoder(w).Encode(config)
	})

	addr := fmt.Sprintf("0.0.0.0:%d", config.Port)
	fmt.Printf("SERVER: Listening on http://localhost:%d\n", config.Port)
	if err := http.ListenAndServe(addr, loggingMux); err != nil {
		fmt.Printf("SERVER FATAL: %v\n", err)
	}
}
