#!/bin/bash
set -e

# --- Environment Variable Defaults ---
NAME=${NAME:-"Garry's Mod Server"}
PASSWORD=${PASSWORD:-""}
RCON_PASSWORD=${RCON_PASSWORD}
MAXPLAYERS=${MAXPLAYERS:-16}
MAP=${MAP:-"gm_construct"}
GAMEMODE=${GAMEMODE:-"sandbox"}
WORKSHOP_ID=${WORKSHOP_ID:-""}
STEAM_AUTHKEY=${STEAM_AUTHKEY:-""}
ARGS=${ARGS:-""}

# --- Construct Launch Command ---
LAUNCH_COMMAND="./srcds_run -game garrysmod -console -usercon -nohltv -ip 0.0.0.0 -port 27015"
LAUNCH_COMMAND+=" +hostname \"$NAME\""
LAUNCH_COMMAND+=" +maxplayers \"$MAXPLAYERS\""
LAUNCH_COMMAND+=" +gamemode \"$GAMEMODE\""
LAUNCH_COMMAND+=" +map \"$MAP\""

if [[ -n "$PASSWORD" ]]; then
  LAUNCH_COMMAND+=" +sv_password \"$PASSWORD\""
fi

if [[ -n "$RCON_PASSWORD" ]]; then
  LAUNCH_COMMAND+=" +rcon_password \"$RCON_PASSWORD\""
fi

if [[ -n "$WORKSHOP_ID" && -n "$STEAM_AUTHKEY" ]]; then
  LAUNCH_COMMAND+=" +host_workshop_collection $WORKSHOP_ID -authkey $STEAM_AUTHKEY"
fi

if [[ -n "$ARGS" ]]; then
  LAUNCH_COMMAND+=" $ARGS"
fi

# --- Launch Server ---
cd /data/server

echo "-> Launching server with the following command:"
echo "$LAUNCH_COMMAND"
echo "-------------------------------------------------"

# Handle shutdown
stop_server() {
    echo "Received SIGTERM, stopping server gracefully..."
    if [[ -n "$RCON_PASSWORD" ]]; then
        /data/scripts/send-command.sh "quit"
    else
        # If no RCON password, just kill the process
        kill -TERM $SERVER_PID 2>/dev/null || true
    fi
    wait $SERVER_PID
    exit 0
}

# Trap SIGTERM
trap stop_server SIGTERM

# Start server in background
$LAUNCH_COMMAND &
SERVER_PID=$!

# Wait for server process
wait $SERVER_PID
