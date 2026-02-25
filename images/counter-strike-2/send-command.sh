#!/bin/bash

RCON_HOST="localhost"
RCON_PASSWORD="${RCON_PASSWORD}"
COMMAND="$1"

if [[ -z "$RCON_PASSWORD" ]]; then
    echo "Error: RCON_PASSWORD not set. Cannot send command."
    exit 1
fi

if [[ -z "$COMMAND" ]]; then
    echo "Error: No command provided."
    exit 1
fi

rcon-cli -a "$RCON_HOST:27015" -p "$RCON_PASSWORD" "$COMMAND"
