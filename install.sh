#!/bin/bash
set -e

# Jail configuration
JAIL_ID="2"
JAIL_PATH="/zroot/sylve/jails/100"
JAIL_BIN="$JAIL_PATH/usr/local/bin"
JAIL_WWW="$JAIL_PATH/usr/local/www/chess"
JAIL_RC="$JAIL_PATH/usr/local/etc/rc.d"

if [[ "$1" == "-jail" ]]; then
    echo "--- Building Grandmaster Chess on Host ---"
    make all

    echo "--- Deploying to Jail $JAIL_ID ($JAIL_PATH) ---"
    
    # Ensure directories exist
    mkdir -p "$JAIL_BIN"
    mkdir -p "$JAIL_WWW"
    mkdir -p "$JAIL_RC"

    # 1. Copy Server Binary & Config
    echo "Installing server to $JAIL_BIN..."
    cp -f chess-server "$JAIL_BIN/"
    cp -f config.json "$JAIL_BIN/"
    chmod 755 "$JAIL_BIN/chess-server"

    # 2. Copy Web Assets
    echo "Installing web assets to $JAIL_WWW..."
    # Clear old web files to ensure a clean update
    rm -rf "$JAIL_WWW"/*
    cp -r web/* "$JAIL_WWW/"
    cp -r assets "$JAIL_WWW/"

    # 3. Create/Update RC Script
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
    chmod 555 "$JAIL_RC/chessui"

    # 4. Restart service inside jail using jexec
    echo "--- Restarting Service inside Jail $JAIL_ID ---"
    jexec $JAIL_ID service chessui restart || echo "Warning: Could not restart service (is it enabled?)"
    
    echo "--- Deployment Complete ---"
    exit 0
fi

# Standard Local Installation
echo "Installing to Local Directory..."
INSTALL_DIR=${1:-"./dist"}
mkdir -p "$INSTALL_DIR"
make all
cp chess-server "$INSTALL_DIR/"
cp config.json "$INSTALL_DIR/"
cp -r web/* "$INSTALL_DIR/web/"
cp -r assets "$INSTALL_DIR/assets/"
echo "Installation complete in $INSTALL_DIR"
