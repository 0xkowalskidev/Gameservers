#!/bin/bash
set -e

# Enable job control for signal handling
set -m

# --- Environment Variable Defaults ---
NAME=${NAME:-"Rust Server"}
PASSWORD=${PASSWORD:-""}
RCON_PASSWORD=${RCON_PASSWORD}
RCON_PORT=${RCON_PORT:-28016}
MAXPLAYERS=${MAXPLAYERS:-50}
WORLDSIZE=${WORLDSIZE:-3000}
SEED=${SEED:-12345}
TICKRATE=${TICKRATE:-30}
SAVEINTERVAL=${SAVEINTERVAL:-300}
ARGS=${ARGS:-""}

# --- Update Server Before Start (if requested) ---
if [[ "$UPDATE_ON_START" == "true" ]]; then
    echo "[$(date)] Updating Rust server before startup..."
    /data/steamcmd/steamcmd.sh +force_install_dir /data/server +login anonymous +app_update 258550 validate +quit
    echo "[$(date)] Server update completed"
fi

# --- Construct Launch Command ---
LAUNCH_COMMAND="./RustDedicated"
LAUNCH_COMMAND+=" -batchmode"
LAUNCH_COMMAND+=" -nographics"
LAUNCH_COMMAND+=" +server.hostname \"$NAME\""
LAUNCH_COMMAND+=" +server.port 28015"
LAUNCH_COMMAND+=" +server.maxplayers $MAXPLAYERS"
LAUNCH_COMMAND+=" +server.worldsize $WORLDSIZE"
LAUNCH_COMMAND+=" +server.seed $SEED"
LAUNCH_COMMAND+=" +server.tickrate $TICKRATE"
LAUNCH_COMMAND+=" +server.saveinterval $SAVEINTERVAL"
LAUNCH_COMMAND+=" +server.identity \"rust-server\""

if [[ -n "$PASSWORD" ]]; then
  LAUNCH_COMMAND+=" +server.password \"$PASSWORD\""
fi

if [[ -n "$RCON_PASSWORD" ]]; then
  LAUNCH_COMMAND+=" +rcon.password \"$RCON_PASSWORD\""
  LAUNCH_COMMAND+=" +rcon.port $RCON_PORT"
  LAUNCH_COMMAND+=" +rcon.web 1"
fi

if [[ -n "$ARGS" ]]; then
  LAUNCH_COMMAND+=" $ARGS"
fi

# --- Launch Server ---
cd /data/server

echo "-> Launching Rust server with the following command:"
echo "$LAUNCH_COMMAND"
echo "-------------------------------------------------"
echo "⚠️  Note: Rust server may take several minutes to start (world generation/loading)"
echo "-------------------------------------------------"

# Handle shutdown
stop_server() {
    echo "[$(date)] Received SIGTERM, stopping Rust server gracefully..." >&2
    if [[ -n "$RCON_PASSWORD" ]]; then
        echo "[$(date)] Sending quit command via RCON..." >&2
        /data/scripts/send-command.sh "quit"
        # Give server time to save and quit gracefully
        sleep 10
    else
        echo "[$(date)] No RCON password set, sending TERM to process..." >&2
        kill -TERM $SERVER_PID 2>/dev/null || true
    fi
    
    # Wait for the server process to exit (with timeout for Rust's long shutdown)
    timeout=60
    count=0
    while kill -0 $SERVER_PID 2>/dev/null && [ $count -lt $timeout ]; do
        echo "[$(date)] Waiting for Rust server to stop... ($count/$timeout)" >&2
        sleep 1
        count=$((count + 1))
    done
    
    # Force kill if still running
    if kill -0 $SERVER_PID 2>/dev/null; then
        echo "[$(date)] Rust server taking too long, force killing..." >&2
        kill -KILL $SERVER_PID 2>/dev/null || true
    fi
    
    echo "[$(date)] Rust server stopped" >&2
    exit 0
}

# Trap SIGTERM and SIGINT
trap stop_server SIGTERM SIGINT

# Start server in background
$LAUNCH_COMMAND &
SERVER_PID=$!

# Wait for server process
echo "[$(date)] Rust server started with PID $SERVER_PID" >&2
echo "[$(date)] Server is starting up... this may take several minutes for world generation" >&2
wait $SERVER_PID