#!/bin/bash

# RCON configuration
RCON_HOST="localhost"
RCON_PORT="27015"
RCON_PASSWORD="${RCON_PASSWORD}"
COMMAND="$1"

# Send command using rcon-cli
rcon-cli -a "$RCON_HOST:$RCON_PORT" -p "$RCON_PASSWORD" "$COMMAND"
