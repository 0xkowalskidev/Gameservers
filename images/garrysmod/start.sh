#!/bin/bash
set -e

# --- Environment Variable Defaults ---
NAME=${NAME:-"T3 Chat GMod Server"}
PASSWORD=${PASSWORD:-""}
RCON_PASSWORD=${RCON_PASSWORD}
MAXPLAYERS=${MAXPLAYERS:-16}
MAP=${MAP:-"gm_construct"}
GAMEMODE=${GAMEMODE:-"sandbox"}
WORKSHOP_ID=${WORKSHOP_ID:-""}
STEAM_AUTHKEY=${STEAM_AUTHKEY:-""}
ARGS=${ARGS:-""}

# --- Construct Launch Command ---
LAUNCH_COMMAND="./srcds_run -game garrysmod -console -usercon -nohltv -ip 0.0.0.0 -port 27015"
LAUNCH_COMMAND+=" +hostname \"$NAME\""
LAUNCH_COMMAND+=" +maxplayers \"$MAXPLAYERS\""
LAUNCH_COMMAND+=" +gamemode \"$GAMEMODE\""
LAUNCH_COMMAND+=" +map \"$MAP\""

if [[ -n "$PASSWORD" ]]; then
  LAUNCH_COMMAND+=" +sv_password \"$PASSWORD\""
fi

if [[ -n "$RCON_PASSWORD" ]]; then
  LAUNCH_COMMAND+=" +rcon_password \"$RCON_PASSWORD\""
fi

if [[ -n "$WORKSHOP_ID" && -n "$STEAM_AUTHKEY" ]]; then
  LAUNCH_COMMAND+=" +host_workshop_collection $WORKSHOP_ID -authkey $STEAM_AUTHKEY"
fi

if [[ -n "$ARGS" ]]; then
  LAUNCH_COMMAND+=" $ARGS"
fi

# --- Launch Server ---
cd /data/server

echo "-> Launching server with the following command:"
echo "$LAUNCH_COMMAND"
echo "-------------------------------------------------"

exec $LAUNCH_COMMAND
