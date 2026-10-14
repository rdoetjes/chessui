#!/bin/bash
set -e

echo "Installing Grandmaster Chess..."

# Create installation directory
INSTALL_DIR=${1:-"./dist"}
mkdir -p "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/web"
mkdir -p "$INSTALL_DIR/assets"

# Build the project
make all

# Copy files
cp chess-server "$INSTALL_DIR/"
cp config.json "$INSTALL_DIR/"
cp -r web/* "$INSTALL_DIR/web/"
cp -r assets/* "$INSTALL_DIR/assets/"

echo "Installation complete in $INSTALL_DIR"
echo "To run the server:"
echo "  cd $INSTALL_DIR && ./chess-server"
