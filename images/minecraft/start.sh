#!/bin/bash

# Enable job control for signal handling
set -m

# Change to server directory
cd /data/server

# Function to download Minecraft server JAR
download_server() {
    local version="${MINECRAFT_VERSION:-latest}"
    local jar_name="minecraft_server_${version}.jar"
    
    echo "[$(date)] Checking Minecraft server version: $version" >&2
    
    # If we already have this version, use it
    if [ -f "$jar_name" ]; then
        echo "[$(date)] Using existing server JAR: $jar_name" >&2
        ln -sf "$jar_name" server.jar
        return 0
    fi
    
    # Fetch version manifest
    echo "[$(date)] Fetching Minecraft version manifest..." >&2
    if ! manifest=$(curl -s https://launchermeta.mojang.com/mc/game/version_manifest_v2.json); then
        echo "[$(date)] ERROR: Failed to fetch version manifest!" >&2
        exit 1
    fi
    
    # If version is "latest", get the latest release version
    if [ "$version" = "latest" ]; then
        # Extract latest release version using jq
        version=$(echo "$manifest" | jq -r '.latest.release')
        if [ -z "$version" ] || [ "$version" = "null" ]; then
            echo "[$(date)] ERROR: Could not determine latest version!" >&2
            exit 1
        fi
        echo "[$(date)] Latest release version is: $version" >&2
        jar_name="minecraft_server_${version}.jar"
        
        # Check again if we have this specific version
        if [ -f "$jar_name" ]; then
            echo "[$(date)] Using existing server JAR: $jar_name" >&2
            ln -sf "$jar_name" server.jar
            return 0
        fi
    fi
    
    # Get the version manifest URL using jq
    version_url=$(echo "$manifest" | jq -r ".versions[] | select(.id == \"$version\") | .url")
    
    if [ -z "$version_url" ] || [ "$version_url" = "null" ]; then
        echo "[$(date)] ERROR: Version $version not found in manifest!" >&2
        echo "[$(date)] Available versions can be found at: https://launchermeta.mojang.com/mc/game/version_manifest_v2.json" >&2
        exit 1
    fi
    
    echo "[$(date)] Found version URL: $version_url" >&2
    
    # Fetch version-specific manifest
    echo "[$(date)] Fetching version manifest for $version..." >&2
    if ! version_manifest=$(curl -s "$version_url"); then
        echo "[$(date)] ERROR: Failed to fetch version-specific manifest!" >&2
        exit 1
    fi
    
    # Extract server download URL using jq
    server_url=$(echo "$version_manifest" | jq -r '.downloads.server.url')
    
    if [ -z "$server_url" ] || [ "$server_url" = "null" ]; then
        echo "[$(date)] ERROR: No server download found for version $version!" >&2
        echo "[$(date)] This version may not have a server JAR available." >&2
        exit 1
    fi
    
    echo "[$(date)] Found server download URL: $server_url" >&2
    
    # Download the server JAR
    echo "[$(date)] Downloading Minecraft server $version..." >&2
    if curl -f -L -o "$jar_name" "$server_url"; then
        echo "[$(date)] Successfully downloaded $jar_name" >&2
        ln -sf "$jar_name" server.jar
        
        # Clean up old versions (keep last 3)
        ls -t minecraft_server_*.jar 2>/dev/null | tail -n +4 | xargs -r rm -f
        echo "[$(date)] Cleaned up old server JARs" >&2
    else
        echo "[$(date)] ERROR: Failed to download server JAR from $server_url!" >&2
        exit 1
    fi
}

# Download/update server JAR
download_server

# Create EULA file based on environment variable
echo "eula=${EULA}" > eula.txt

# Update server.properties with environment variables
if [ ! -z "$SERVER_NAME" ]; then
    sed -i "s/motd=.*/motd=${SERVER_NAME}/" server.properties
fi

if [ ! -z "$MOTD" ]; then
    sed -i "s/motd=.*/motd=${MOTD}/" server.properties
fi

if [ ! -z "$DIFFICULTY" ]; then
    sed -i "s/difficulty=.*/difficulty=${DIFFICULTY}/" server.properties
fi

if [ ! -z "$GAMEMODE" ]; then
    sed -i "s/gamemode=.*/gamemode=${GAMEMODE}/" server.properties
fi

# Set consistent ports (server port should match container port)
sed -i "s/server-port=.*/server-port=25565/" server.properties
sed -i "s/rcon.port=.*/rcon.port=25575/" server.properties

# Create named pipe for command input
PIPE_PATH="/tmp/command-fifo"
mkfifo "$PIPE_PATH"

# Handle shutdown
stop_server() {
    echo "[$(date)] Received SIGTERM, stopping Minecraft server gracefully..." >&2
    echo "stop" > $PIPE_PATH
    echo "[$(date)] Stop command sent to Minecraft server" >&2
    # Wait for the specific Java process to exit
    while kill -0 $SERVER_PID 2>/dev/null; do
        echo "[$(date)] Waiting for Minecraft server to stop..." >&2
        sleep 1
    done
    echo "[$(date)] Minecraft server stopped gracefully" >&2
    exit 0
}

# Trap SIGTERM and SIGINT
trap stop_server SIGTERM SIGINT

# Start server in background and get PID
while true; do
  cat $PIPE_PATH
done | java -Xmx${MEMORY_MB}M -Xms${MEMORY_MB}M -jar server.jar nogui &
SERVER_PID=$!

# Wait for server process
echo "[$(date)] Minecraft server started with PID $SERVER_PID" >&2
wait $SERVER_PID