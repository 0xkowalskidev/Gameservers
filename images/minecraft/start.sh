#!/bin/bash

# Trap SIGTERM and run stop script
trap '/data/scripts/stop.sh' SIGTERM

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

# Start server 
while true; do
  cat $PIPE_PATH
done | java -Xmx${MEMORY_MB}M -Xms${MEMORY_MB}M -jar server.jar nogui
