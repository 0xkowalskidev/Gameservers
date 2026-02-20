#!/bin/bash

# RCON configuration
RCON_HOST="localhost"
RCON_PORT="${RCON_PORT:-27020}"
RCON_PASSWORD="${ADMIN_PASSWORD}"
COMMAND="$1"

# Check if RCON password is set
if [[ -z "$RCON_PASSWORD" ]]; then
    echo "Error: ADMIN_PASSWORD not set. Cannot send command."
    exit 1
fi

# Check if command is provided
if [[ -z "$COMMAND" ]]; then
    echo "Error: No command provided."
    echo "Usage: $0 <command>"
    exit 1
fi

# Send command using rcon-cli
rcon-cli -a "$RCON_HOST:$RCON_PORT" -p "$RCON_PASSWORD" "$COMMAND"
