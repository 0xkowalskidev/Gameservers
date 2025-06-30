#!/bin/bash

# RCON configuration
RCON_HOST="localhost"
RCON_PASSWORD="${RCON_PASSWORD}"
COMMAND="$1"

# Check if RCON password is set
if [[ -z "$RCON_PASSWORD" ]]; then
    echo "Error: RCON_PASSWORD not set. Cannot send command."
    exit 1
fi

# Check if command is provided
if [[ -z "$COMMAND" ]]; then
    echo "Error: No command provided."
    echo "Usage: $0 <command>"
    exit 1
fi

# Send command using rcon-cli
rcon-cli -a "$RCON_HOST:28016" -p "$RCON_PASSWORD" "$COMMAND"
