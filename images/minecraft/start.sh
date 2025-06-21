#!/bin/bash

# Change to server directory
cd /data/server

# Create EULA file based on environment variable
echo "eula=${EULA}" > eula.txt

# Create named pipe for command input
PIPE_PATH="/tmp/command-fifo"
mkfifo "$PIPE_PATH"

# Start server with configurable memory, using named pipe as input
exec java -Xmx${MEMORY_MB}M -Xms${MEMORY_MB}M -jar server.jar nogui < "$PIPE_PATH"
