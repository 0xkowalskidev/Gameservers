#!/bin/bash

# --- Update Server ---
echo "-> Updating Valheim server via SteamCMD..."
steamcmd +force_install_dir /data/server +login anonymous +app_update 896660 validate +quit

cd /data/server

export templdpath=$LD_LIBRARY_PATH
export LD_LIBRARY_PATH=./linux64:$LD_LIBRARY_PATH
export SteamAppID=892970

# Set default values
SERVER_NAME="${SERVER_NAME:-My Valheim Server}"
PASSWORD="${PASSWORD:-valheim123}"
PUBLIC="${PUBLIC:-1}"
CROSSPLAY="${CROSSPLAY:-1}"

echo "[$(date)] Starting Valheim server: ${SERVER_NAME}"



# Build arguments array to handle spaces properly
ARGS=(
    -name "${SERVER_NAME}"
    -port 2456
    -public "${PUBLIC}"
    -world "world"
    -password "${PASSWORD}"
    -batchmode
    -nographics
)

# Add crossplay flag if enabled
if [ "${CROSSPLAY}" = "1" ]; then
    ARGS+=(-crossplay)
fi

# Start the actual server
./valheim_server.x86_64 "${ARGS[@]}"
