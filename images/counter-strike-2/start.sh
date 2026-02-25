#!/bin/bash
set -e
set -m

# Setup steamclient.so symlink (required for CS2)
mkdir -p ~/.steam/sdk64
ln -sf /data/steamcmd/linux64/steamclient.so ~/.steam/sdk64/steamclient.so 2>/dev/null || true

# --- Update Server ---
echo "-> Updating CS2 server via SteamCMD..."
/data/steamcmd/steamcmd.sh +force_install_dir /data/server +login anonymous +app_update 730 validate +quit

# --- Environment Variable Defaults ---
NAME=${NAME:-"CS2 Server"}
PASSWORD=${PASSWORD:-""}
RCON_PASSWORD=${RCON_PASSWORD}
MAXPLAYERS=${MAXPLAYERS:-16}
GAMEMODE=${GAMEMODE:-"competitive"}
MAP=${MAP:-"de_dust2"}
GSLT=${GSLT:-""}
ARGS=${ARGS:-""}

# --- Construct Launch Command ---
cd /data/server/game

LAUNCH_COMMAND="./cs2.sh -dedicated -port 27015"
LAUNCH_COMMAND+=" -maxplayers $MAXPLAYERS"
LAUNCH_COMMAND+=" +hostname \"$NAME\""
LAUNCH_COMMAND+=" +game_alias $GAMEMODE"
LAUNCH_COMMAND+=" +map $MAP"

if [[ -n "$PASSWORD" ]]; then
    LAUNCH_COMMAND+=" +sv_password \"$PASSWORD\""
fi

if [[ -n "$RCON_PASSWORD" ]]; then
    LAUNCH_COMMAND+=" +rcon_password \"$RCON_PASSWORD\""
fi

if [[ -n "$GSLT" ]]; then
    LAUNCH_COMMAND+=" +sv_setsteamaccount $GSLT"
fi

if [[ -n "$ARGS" ]]; then
    LAUNCH_COMMAND+=" $ARGS"
fi

# --- Launch Server ---
echo "-> Launching CS2 server with command:"
echo "$LAUNCH_COMMAND"
echo "-------------------------------------------------"

# Handle shutdown
stop_server() {
    echo "[$(date)] Received SIGTERM, stopping CS2 server gracefully..." >&2
    if [[ -n "$RCON_PASSWORD" ]]; then
        echo "[$(date)] Sending quit command via RCON..." >&2
        /data/scripts/send-command.sh "quit"
    else
        echo "[$(date)] No RCON password set, sending TERM to process..." >&2
        kill -TERM $SERVER_PID 2>/dev/null || true
    fi
    while kill -0 $SERVER_PID 2>/dev/null; do
        echo "[$(date)] Waiting for CS2 server to stop..." >&2
        sleep 1
    done
    echo "[$(date)] CS2 server stopped gracefully" >&2
    exit 0
}

trap stop_server SIGTERM SIGINT

# Start server in background
$LAUNCH_COMMAND &
SERVER_PID=$!

echo "[$(date)] CS2 server started with PID $SERVER_PID" >&2
wait $SERVER_PID
