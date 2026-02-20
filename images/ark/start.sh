#!/bin/bash

# --- Update Server ---
echo "-> Updating ARK server via SteamCMD..."
steamcmd +force_install_dir /data/server +login anonymous +app_update 376030 validate +quit

# --- Environment Variable Defaults ---
SERVER_NAME=${SERVER_NAME:-"ARK Server"}
SERVER_PASSWORD=${SERVER_PASSWORD:-""}
ADMIN_PASSWORD=${ADMIN_PASSWORD:-""}
MAX_PLAYERS=${MAX_PLAYERS:-70}
MAP_NAME=${MAP_NAME:-"TheIsland"}
DIFFICULTY=${DIFFICULTY:-"1.0"}
RCON_ENABLED=${RCON_ENABLED:-"True"}
RCON_PORT=${RCON_PORT:-27020}

# --- Build server arguments ---
# ARK uses URL-style parameters for configuration
SERVER_OPTS="${MAP_NAME}"
SERVER_OPTS+="?SessionName=${SERVER_NAME}"
SERVER_OPTS+="?QueryPort=27015"
SERVER_OPTS+="?MaxPlayers=${MAX_PLAYERS}"
SERVER_OPTS+="?DifficultyOffset=${DIFFICULTY}"

if [[ -n "$SERVER_PASSWORD" ]]; then
    SERVER_OPTS+="?ServerPassword=${SERVER_PASSWORD}"
fi

if [[ -n "$ADMIN_PASSWORD" ]]; then
    SERVER_OPTS+="?ServerAdminPassword=${ADMIN_PASSWORD}"
    SERVER_OPTS+="?RCONEnabled=${RCON_ENABLED}"
    SERVER_OPTS+="?RCONPort=${RCON_PORT}"
fi

# --- Launch Server ---
echo "-> Launching ARK server with configuration:"
echo "   Map: ${MAP_NAME}"
echo "   Server Name: ${SERVER_NAME}"
echo "   Max Players: ${MAX_PLAYERS}"
echo "   Difficulty: ${DIFFICULTY}"
echo "   RCON Enabled: ${RCON_ENABLED}"
echo "-------------------------------------------------"
echo "Note: ARK server may take several minutes to start (loading assets)"
echo "-------------------------------------------------"

/data/server/ShooterGame/Binaries/Linux/ShooterGameServer "${SERVER_OPTS}" -server -log 
