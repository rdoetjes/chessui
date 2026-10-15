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
        echo ">>> STARTING JAIL DEPLOYMENT (v5.18)"
        echo ">>> Building binaries on host machine..."
        make all

        echo ">>> Ensuring jail directories exist..."
        mkdir -p "${JAIL_BIN}"
        mkdir -p "${JAIL_WWW}"
        mkdir -p "${JAIL_RC}"

        echo ">>> DEPLOYING SERVER: Copying binary to ${JAIL_BIN}..."
        cp -f chess-server "${JAIL_BIN}/"
        cp -f config.json "${JAIL_BIN}/"
        chmod 755 "${JAIL_BIN}/chess-server"

        echo ">>> DEPLOYING WEB: Copying assets to ${JAIL_WWW}..."
        # Clear old web files to ensure a clean update
        rm -rf "${JAIL_WWW:?}"/*
        # Copy web CONTENTS directly to JAIL_WWW root
        cp -r web/* "${JAIL_WWW}/"
        # Copy assets folder to JAIL_WWW/assets
        cp -r assets "${JAIL_WWW}/"
        
        # Ensure correct permissions
        chown -R 80:80 "${JAIL_WWW}" || echo "Note: Could not set ownership"

        echo ">>> INSTALLING SERVICE: Creating /usr/local/etc/rc.d/chess-server..."
        cat > "${JAIL_RC}/chess-server" <<EOF
#!/bin/sh
#
# PROVIDE: chess_server
# REQUIRE: LOGIN
# KEYWORD: shutdown

. /etc/rc.subr

name="chess_server"
rcvar="chess_server_enable"

command="/usr/local/bin/chess-server"
# The -web flag now points to the JAIL_WWW directly
command_args="-web /usr/local/www/chess"
pidfile="/var/run/\${name}.pid"

# Run as caddy user inside the jail
chess_server_user="caddy"

start_cmd="chess_server_start"

chess_server_start()
{
    echo "Starting Grandmaster Chess Server..."
    cd /usr/local/bin
    /usr/sbin/daemon -f -p \${pidfile} -u \${chess_server_user} \${command} \${command_args}
}

load_rc_config \$name
run_rc_command "\$1"
EOF
        chmod 555 "${JAIL_RC}/chess-server"

        echo ">>> RESTARTING: Executing jexec ${JAIL_ID} service chess-server restart..."
        jexec "${JAIL_ID}" service chess-server restart || echo "Warning: Could not restart service"
        
        echo ">>> DEPLOYMENT COMPLETE (v5.18) <<<"
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
