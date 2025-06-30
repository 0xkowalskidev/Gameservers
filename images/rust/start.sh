#!/bin/bash
# Set up library path for Rust server
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/data/server/RustDedicated_Data/Plugins:/data/server/RustDedicated_Data/Plugins/x86_64

# --- Environment Variable Defaults ---
NAME=${NAME:-"Rust Server"}
PASSWORD=${PASSWORD:-""}
RCON_PASSWORD=${RCON_PASSWORD}
RCON_PORT=${RCON_PORT:-28016}
MAXPLAYERS=${MAXPLAYERS:-50}
WORLDSIZE=${WORLDSIZE:-1000}
SEED=${SEED:-12345}
TICKRATE=${TICKRATE:-30}
SAVEINTERVAL=${SAVEINTERVAL:-300}
ARGS=${ARGS:-""}

# --- Build server arguments ---
SERVER_ARGS=()
SERVER_ARGS+=("-batchmode")
SERVER_ARGS+=("+server.hostname" "$NAME")
SERVER_ARGS+=("+server.port" "28015")
SERVER_ARGS+=("+server.ip" "0.0.0.0")
SERVER_ARGS+=("+server.maxplayers" "$MAXPLAYERS")
SERVER_ARGS+=("+server.worldsize" "$WORLDSIZE")
SERVER_ARGS+=("+server.seed" "$SEED")
SERVER_ARGS+=("+server.tickrate" "$TICKRATE")
SERVER_ARGS+=("+server.saveinterval" "$SAVEINTERVAL")
SERVER_ARGS+=("+server.identity" "rust-server")

if [[ -n "$PASSWORD" ]]; then
    SERVER_ARGS+=("+server.password" "$PASSWORD")
fi

if [[ -n "$RCON_PASSWORD" ]]; then
    SERVER_ARGS+=("+rcon.password" "$RCON_PASSWORD")
    SERVER_ARGS+=("+rcon.port" "$RCON_PORT")
    SERVER_ARGS+=("+rcon.web" "1")
fi

# Add any additional arguments
if [[ -n "$ARGS" ]]; then
    SERVER_ARGS+=($ARGS)
fi

# --- Launch Server ---
echo "-> Launching Rust server with arguments:"
echo "   ${SERVER_ARGS[@]}"
echo "-------------------------------------------------"
echo "⚠️  Note: Rust server may take several minutes to start (world generation/loading)"
echo "-------------------------------------------------"

/data/server/RustDedicated "${SERVER_ARGS[@]}"
