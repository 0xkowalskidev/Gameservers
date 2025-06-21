#!/bin/bash

# Change to server directory
cd /data/server

# Create EULA file based on environment variable
echo "eula=${EULA}" > eula.txt

# Start server with configurable memory
exec java -Xmx${MEMORY_MB}M -Xms${MEMORY_MB}M -jar server.jar nogui
