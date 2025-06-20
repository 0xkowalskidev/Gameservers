#!/bin/bash

# Create EULA file based on environment variable
echo "eula=${EULA}" > eula.txt

# Start server with configurable memory
exec java -Xmx${MAX_MEMORY} -Xms${MIN_MEMORY} -jar server.jar nogui