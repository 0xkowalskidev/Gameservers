#!/bin/bash

# Enable job control for signal handling
set -m

# Change to server directory
cd /data/server

# Resolved version (set by download_server)
RESOLVED_VERSION=""

# Function to get correct Java path for Minecraft version
get_java_for_version() {
    local mc_version="$1"
    local major minor

    # Parse version (e.g., "1.20.4" -> major=20, minor=4)
    major=$(echo "$mc_version" | cut -d'.' -f2)
    minor=$(echo "$mc_version" | cut -d'.' -f3)

    # Default minor to 0 if not present
    minor=${minor:-0}

    if [ "$major" -ge 21 ] || ([ "$major" -eq 20 ] && [ "$minor" -ge 5 ]); then
        # 1.20.5+ requires Java 21
        echo "/usr/lib/jvm/java-21-openjdk-amd64"
    elif [ "$major" -ge 17 ]; then
        # 1.17 - 1.20.4 requires Java 17
        echo "/usr/lib/jvm/java-17-openjdk-amd64"
    else
        # 1.16 and below requires Java 8
        echo "/usr/lib/jvm/java-8-openjdk-amd64"
    fi
}

# Function to download Minecraft server JAR
download_server() {
    local version="${MINECRAFT_VERSION:-latest}"
    local jar_name="minecraft_server_${version}.jar"

    echo "[$(date)] Checking Minecraft server version: $version" >&2

    # If we already have this version, use it
    if [ -f "$jar_name" ]; then
        echo "[$(date)] Using existing server JAR: $jar_name" >&2
        ln -sf "$jar_name" server.jar
        RESOLVED_VERSION="$version"
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
            RESOLVED_VERSION="$version"
            return 0
        fi
    fi

    # Store resolved version
    RESOLVED_VERSION="$version"

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

# Select correct Java version based on Minecraft version
JAVA_HOME=$(get_java_for_version "$RESOLVED_VERSION")
export JAVA_HOME
export PATH="$JAVA_HOME/bin:$PATH"
echo "[$(date)] Using Java from: $JAVA_HOME" >&2
echo "[$(date)] Java version: $($JAVA_HOME/bin/java -version 2>&1 | head -1)" >&2

# Create EULA file based on environment variable
echo "eula=${EULA}" > eula.txt

# Update server.properties with environment variables
if [ -n "$SERVER_NAME" ]; then
    sed -i "s/motd=.*/motd=${SERVER_NAME}/" server.properties
fi

if [ -n "$MOTD" ]; then
    sed -i "s/motd=.*/motd=${MOTD}/" server.properties
fi

if [ -n "$DIFFICULTY" ]; then
    sed -i "s/difficulty=.*/difficulty=${DIFFICULTY}/" server.properties
fi

if [ -n "$GAMEMODE" ]; then
    sed -i "s/gamemode=.*/gamemode=${GAMEMODE}/" server.properties
fi

if [ -n "$MAX_PLAYERS" ]; then
    sed -i "s/max-players=.*/max-players=${MAX_PLAYERS}/" server.properties
fi

if [ -n "$VIEW_DISTANCE" ]; then
    sed -i "s/view-distance=.*/view-distance=${VIEW_DISTANCE}/" server.properties
fi

if [ -n "$PVP" ]; then
    sed -i "s/pvp=.*/pvp=${PVP}/" server.properties
fi

if [ -n "$WHITELIST" ]; then
    sed -i "s/white-list=.*/white-list=${WHITELIST}/" server.properties
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

# Aikar's optimized JVM flags for Minecraft servers
# https://docs.papermc.io/paper/aikars-flags
AIKAR_FLAGS="-XX:+UseG1GC -XX:+ParallelRefProcEnabled -XX:MaxGCPauseMillis=200"
AIKAR_FLAGS="$AIKAR_FLAGS -XX:+UnlockExperimentalVMOptions -XX:+DisableExplicitGC"
AIKAR_FLAGS="$AIKAR_FLAGS -XX:+AlwaysPreTouch -XX:G1NewSizePercent=30 -XX:G1MaxNewSizePercent=40"
AIKAR_FLAGS="$AIKAR_FLAGS -XX:G1HeapRegionSize=8M -XX:G1ReservePercent=20 -XX:G1HeapWastePercent=5"
AIKAR_FLAGS="$AIKAR_FLAGS -XX:G1MixedGCCountTarget=4 -XX:InitiatingHeapOccupancyPercent=15"
AIKAR_FLAGS="$AIKAR_FLAGS -XX:G1MixedGCLiveThresholdPercent=90 -XX:G1RSetUpdatingPauseTimePercent=5"
AIKAR_FLAGS="$AIKAR_FLAGS -XX:SurvivorRatio=32 -XX:+PerfDisableSharedMem -XX:MaxTenuringThreshold=1"
AIKAR_FLAGS="$AIKAR_FLAGS -Dusing.aikars.flags=https://mcflags.emc.gs -Daikars.new.flags=true"

echo "[$(date)] Starting Minecraft server with Aikar's optimized flags" >&2
echo "[$(date)] Memory: ${MEMORY_MB}MB" >&2

# Start server in background and get PID
while true; do
  cat $PIPE_PATH
done | "$JAVA_HOME/bin/java" -Xms${MEMORY_MB}M -Xmx${MEMORY_MB}M $AIKAR_FLAGS -jar server.jar nogui &
SERVER_PID=$!

# Wait for server process
echo "[$(date)] Minecraft server started with PID $SERVER_PID" >&2
wait $SERVER_PID
