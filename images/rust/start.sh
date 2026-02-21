#!/bin/bash
# Set up library path for Rust server
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/data/server/RustDedicated_Data/Plugins:/data/server/RustDedicated_Data/Plugins/x86_64

# --- Update Server ---
UPDATE_ON_START=${UPDATE_ON_START:-"false"}

if [[ ! -f "/data/server/RustDedicated" ]]; then
    echo "-> Installing Rust server via SteamCMD (first run)..."
    steamcmd +force_install_dir /data/server +login anonymous +app_update 258550 validate +quit
elif [[ "$UPDATE_ON_START" == "true" || "$UPDATE_ON_START" == "1" ]]; then
    echo "-> Updating Rust server via SteamCMD..."
    # Note: Don't use 'validate' flag to avoid wiping Oxide/plugin files
    steamcmd +force_install_dir /data/server +login anonymous +app_update 258550 +quit
else
    echo "-> Skipping server update (set UPDATE_ON_START=true to update)"
fi

# --- Install Enabled Mods ---
if [[ -n "$ENABLED_MODS" ]]; then
    echo "-> Installing enabled mods: $ENABLED_MODS"
    IFS=',' read -ra MODS <<< "$ENABLED_MODS"
    for mod in "${MODS[@]}"; do
        mod=$(echo "$mod" | tr -d ' ')  # trim whitespace
        if [[ -n "$mod" && -f "/data/scripts/mods/${mod}/install.sh" ]]; then
            echo "   Installing mod: $mod"
            bash "/data/scripts/mods/${mod}/install.sh"
        else
            echo "   Warning: Mod script not found for: $mod"
        fi
    done
fi

# --- Environment Variable Defaults ---
NAME=${NAME:-"Rust Server"}
PASSWORD=${PASSWORD:-""}
RCON_PASSWORD=${RCON_PASSWORD}
MAXPLAYERS=${MAXPLAYERS:-50}
WORLDSIZE=${WORLDSIZE:-1000}
SEED=${SEED:-12345}
TICKRATE=${TICKRATE:-30}
SAVEINTERVAL=${SAVEINTERVAL:-300}
SERVER_SECURE=${SERVER_SECURE:-"1"}
SERVER_ENCRYPTION=${SERVER_ENCRYPTION:-"1"}
SERVER_EAC=${SERVER_EAC:-"1"}
ARGS=${ARGS:-""}

# --- Build server arguments ---
SERVER_ARGS=()
SERVER_ARGS+=("-batchmode")
SERVER_ARGS+=("+server.hostname" "$NAME")
SERVER_ARGS+=("+server.port" "28015")
SERVER_ARGS+=("+server.queryport" "28017")
SERVER_ARGS+=("+server.ip" "0.0.0.0")
SERVER_ARGS+=("+server.maxplayers" "$MAXPLAYERS")
SERVER_ARGS+=("+server.worldsize" "$WORLDSIZE")
SERVER_ARGS+=("+server.seed" "$SEED")
SERVER_ARGS+=("+server.tickrate" "$TICKRATE")
SERVER_ARGS+=("+server.saveinterval" "$SAVEINTERVAL")
SERVER_ARGS+=("+server.identity" "rust-server")
SERVER_ARGS+=("+server.secure" "$SERVER_SECURE")
SERVER_ARGS+=("+server.encryption" "$SERVER_ENCRYPTION")
SERVER_ARGS+=("+server.eac" "$SERVER_EAC")

if [[ -n "$PASSWORD" ]]; then
    SERVER_ARGS+=("+server.password" "$PASSWORD")
fi

if [[ -n "$RCON_PASSWORD" ]]; then
    SERVER_ARGS+=("+rcon.password" "$RCON_PASSWORD")
    SERVER_ARGS+=("+rcon.port" 28016)
    SERVER_ARGS+=("+rcon.web" "0")
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
