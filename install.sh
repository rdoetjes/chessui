#!/bin/bash
set -e

JAIL_ROOT="/zroot/sylve/jails/100"
JAIL_WWW="$JAIL_ROOT/usr/local/www/chess"
JAIL_BIN="$JAIL_ROOT/usr/local/bin"
JAIL_RC="$JAIL_ROOT/usr/local/etc/rc.d"

if [[ "$1" == "-jail" ]]; then
    echo "Installing Grandmaster Chess to Jail..."
    
    # 1. Build locally on host
    make all
    
    # 2. Create jail directories if they don't exist
    mkdir -p "$JAIL_WWW"
    mkdir -p "$JAIL_BIN"
    mkdir -p "$JAIL_RC"

    # 3. Copy Web Assets
    echo "Copying web assets to $JAIL_WWW..."
    cp -r web/* "$JAIL_WWW/"
    cp -r assets "$JAIL_WWW/"

    # 4. Copy Server Binary & Config
    echo "Copying server to $JAIL_BIN..."
    cp chess-server "$JAIL_BIN/"
    cp config.json "$JAIL_BIN/"

    # 5. Create and Copy RC Script
    echo "Installing RC script to $JAIL_RC/chessui..."
    cat > "$JAIL_RC/chessui" <<EOF
#!/bin/sh
#
# PROVIDE: chessui
# REQUIRE: LOGIN
# KEYWORD: shutdown

. /etc/rc.subr

name="chessui"
rcvar="chessui_enable"

chessui_dir="/usr/local/www/chess"
command="/usr/local/bin/chess-server"
pidfile="/var/run/\${name}.pid"

# Run as caddy user inside the jail
chessui_user="caddy"

start_cmd="chessui_start"

chessui_start()
{
    echo "Starting Grandmaster Chess Server..."
    cd \${chessui_dir}
    /usr/sbin/daemon -f -p \${pidfile} -u \${chessui_user} \${command}
}

load_rc_config \$name
run_rc_command "\$1"
EOF
    chmod +x "$JAIL_RC/chessui"

    echo "Jail Installation Complete!"
    echo "Inside the jail, run: sysrc chessui_enable=YES && service chessui start"
    exit 0
fi

echo "Local Installation..."
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
