#!/bin/bash

# Default values if environment variables are not set
: "${MAXPLAYERS:=8}"
: "${PASSWORD:=}"
: "${WORLD:=world.wld}"
: "${DIFFICULTY:=0}"

# Start the server with command-line arguments
mono /data/server/TerrariaServer.exe \
  -port 7777 \
  -world "/data/server/${WORLD}" \
  -autocreate 1 \
  -worldname "TerrariaWorld" \
  -maxplayers "${MAXPLAYERS}" \
  -password "${PASSWORD}" \
  -difficulty "${DIFFICULTY}" \
  -logfile /data/server/server.log
