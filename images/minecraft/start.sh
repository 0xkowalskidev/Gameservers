#!/bin/bash

# Enable job control for signal handling
set -m

# Change to server directory
cd /data/server

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
