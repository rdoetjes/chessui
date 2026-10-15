#!/bin/sh
set -e

# Jail configuration
JAIL_ID="2"
JAIL_PATH="/zroot/sylve/jails/100"
JAIL_BIN="${JAIL_PATH}/usr/local/bin"
JAIL_WWW="${JAIL_PATH}/usr/local/www/chess"
JAIL_RC="${JAIL_PATH}/usr/local/etc/rc.d"

case "$1" in
    -jail|jail)
        echo ">>> STARTING JAIL DEPLOYMENT (v5.10)"
        echo ">>> Building binaries on host..."
        make all

        echo ">>> Creating directories in jail..."
        mkdir -p "${JAIL_BIN}"
        mkdir -p "${JAIL_WWW}"
        mkdir -p "${JAIL_RC}"

        echo ">>> Copying server to ${JAIL_BIN}..."
        cp -f chess-server "${JAIL_BIN}/"
        cp -f config.json "${JAIL_BIN}/"
        chmod 755 "${JAIL_BIN}/chess-server"

        echo ">>> Copying web assets to ${JAIL_WWW}..."
        # Clear old web files to ensure a clean update
        rm -rf "${JAIL_WWW:?}"/*
        # Copy the folders themselves, not just contents
        cp -r web "${JAIL_WWW}/"
        cp -r assets "${JAIL_WWW}/"

        echo ">>> Installing RC script to ${JAIL_RC}/chess-server..."
        cat > "${JAIL_RC}/chess-server" <<EOF
#!/bin/sh
#
# PROVIDE: chess_server
# REQUIRE: LOGIN
# KEYWORD: shutdown

. /etc/rc.subr

name="chess_server"
rcvar="chess_server_enable"

chess_server_dir="/usr/local/www/chess"
command="/usr/local/bin/chess-server"
pidfile="/var/run/\${name}.pid"

# Run as caddy user inside the jail
chess_server_user="caddy"

start_cmd="chess_server_start"

chess_server_start()
{
    echo "Starting Grandmaster Chess Server..."
    cd \${chess_server_dir}
    /usr/sbin/daemon -f -p \${pidfile} -u \${chess_server_user} \${command}
}

load_rc_config \$name
run_rc_command "\$1"
EOF
        chmod 555 "${JAIL_RC}/chess-server"

        echo ">>> Restarting Service inside Jail ${JAIL_ID} ---"
        jexec "${JAIL_ID}" service chess-server restart || echo "Warning: Could not restart service (is it enabled?)"
        
        echo ">>> DEPLOYMENT COMPLETE <<<"
        echo "Note: Ensure 'chess_server_enable=\"YES\"' is in the jail's /etc/rc.conf"
        exit 0
        ;;
    *)
        echo "Installing to Local Directory..."
        INSTALL_DIR=${1:-"./dist"}
        mkdir -p "$INSTALL_DIR"
        make all
        cp chess-server "$INSTALL_DIR/"
        cp config.json "$INSTALL_DIR/"
        cp -r web/* "$INSTALL_DIR/web/"
        cp -r assets "$INSTALL_DIR/assets/"
        echo "Installation complete in $INSTALL_DIR"
        ;;
esac
