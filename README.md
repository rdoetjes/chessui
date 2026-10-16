# ![Chess Pieces](assets/pieces/standard/wq.png) Grandmaster Chess UI

A professional, high-performance Chess interface powered by **Go**, **WebAssembly**, and **Stockfish**. This project provides a sleek, modern experience for playing chess against an engine or other humans, featuring real-time move analysis and multiple visual themes.

![Chess Pieces](assets/pieces/standard/wk.png) ![Chess Pieces](assets/pieces/standard/bk.png)

## 🚀 Features

### 🧠 Advanced Engine Integration
*   **Real-time Evaluation**: Integrated Stockfish engine providing instant centipawn evaluation.
*   **Move Ratings**: Automatic detection of **Blunders (🔴)**, **Inaccuracies (🟠)**, and **Excellent Moves (🟢)**.
*   **Adaptive Strength**: Choose your challenge with bot levels ranging from ELO 100 to 2700+.

### 🎮 Game Modes
*   **Play vs Bot**: Test your skills against a configurable Stockfish engine.
*   **Multiplayer**: Host or join games using unique session IDs for head-to-head play.
*   **Setup Mode**: Create custom board positions for analysis or specialized training.

### 🎨 Modern Interface
*   **Dynamic Themes**: Toggle between **Standard**, **Classic** (Wood/Gold), and **Modern** (Dark/Neon) styles.
*   **Interactive Evaluation Bar**: Visual feedback on who has the advantage.
*   **Captured Pieces**: Real-time tracking of material advantage and captured pieces.
*   **PGN Export**: Easily copy your games in standard PGN format for analysis elsewhere.

---

## 🛠️ Technology Stack

*   **Logic**: [Go](https://go.dev/) compiled to **WebAssembly (WASM)** for near-native performance in the browser.
*   **Backend**: High-concurrency Go server managing WebSocket communication and Stockfish process isolation.
*   **Frontend**: Vanilla HTML5/CSS3 with a responsive Canvas-based board renderer.
*   **Protocol**: UCI (Universal Chess Interface) over WebSockets.

---

## 📦 Installation & Setup

### Prerequisites
*   [Go](https://go.dev/doc/install) (1.21+)
*   [Stockfish](https://stockfishchess.org/download/) installed and available in your PATH.

### 1. Clone the Repository
```bash
git clone https://github.com/yourusername/chess-ui.git
cd chess-ui
```

### 2. Configure the Engine
Edit `config.json` to match your environment:
```json
{
    "stockfish_path": "stockfish",
    "threads": 8,
    "port": 8081
}
```

### 3. Build & Run
You can use the provided `Makefile` for a streamlined setup:

```bash
# Build both WASM logic and Server
make all

# Start the server
./chess-server
```

The application will be available at `http://localhost:8081`.

### 🐳 Deployment (FreeBSD/Jail)
The project includes a specialized `install.sh` script for deploying into FreeBSD jail (set default to id 2 as this was intended as just a home project):
```bash
./install.sh jail
```

---

## 📂 Project Structure

*   `cmd/server/`: Backend Go server entry point.
*   `cmd/wasm/`: Frontend logic written in Go (compiled to `main.wasm`).
*   `internal/uci/`: UCI protocol parser and engine communication logic.
*   `web/`: Static assets, CSS, and the HTML entry point.
*   `assets/pieces/`: high-quality piece sets for different themes.

---

## 🤝 Contributing

Contributions are welcome! Whether it's bug fixes, new themes, or feature requests, feel free to open an issue or submit a pull request.

---

*Designed with precision for the modern chess enthusiast.*
