#!/bin/bash
set -e

# --- Environment Variable Defaults ---
GMOD_HOSTNAME=${GMOD_HOSTNAME:-"T3 Chat GMod Server"}
GMOD_PASSWORD=${GMOD_PASSWORD:-""}
GMOD_RCON_PASSWORD=${GMOD_RCON_PASSWORD:-"changeme"}
GMOD_MAXPLAYERS=${GMOD_MAXPLAYERS:-16}
GMOD_MAP=${GMOD_MAP:-"gm_construct"}
GMOD_GAMEMODE=${GMOD_GAMEMODE:-"sandbox"}
GMOD_WORKSHOP_ID=${GMOD_WORKSHOP_ID:-""}
STEAM_AUTHKEY=${STEAM_AUTHKEY:-""}
GMOD_ARGS=${GMOD_ARGS:-""}

# --- Construct Launch Command ---
LAUNCH_COMMAND="./srcds_run -game garrysmod -console -usercon -nohltv -ip 0.0.0.0 -port 27015"
LAUNCH_COMMAND+=" +hostname \"$GMOD_HOSTNAME\""
LAUNCH_COMMAND+=" +maxplayers \"$GMOD_MAXPLAYERS\""
LAUNCH_COMMAND+=" +gamemode \"$GMOD_GAMEMODE\""
LAUNCH_COMMAND+=" +map \"$GMOD_MAP\""

if [[ -n "$GMOD_PASSWORD" ]]; then
  LAUNCH_COMMAND+=" +sv_password \"$GMOD_PASSWORD\""
fi

if [[ -n "$GMOD_RCON_PASSWORD" ]]; then
  LAUNCH_COMMAND+=" +rcon_password \"$GMOD_RCON_PASSWORD\""
fi

if [[ -n "$GMOD_WORKSHOP_ID" && -n "$STEAM_AUTHKEY" ]]; then
  LAUNCH_COMMAND+=" +host_workshop_collection $GMOD_WORKSHOP_ID -authkey $STEAM_AUTHKEY"
fi

if [[ -n "$GMOD_ARGS" ]]; then
  LAUNCH_COMMAND+=" $GMOD_ARGS"
fi

# --- Launch Server ---
cd /data/server

echo "-> Launching server with the following command:"
echo "$LAUNCH_COMMAND"
echo "-------------------------------------------------"

exec $LAUNCH_COMMAND
