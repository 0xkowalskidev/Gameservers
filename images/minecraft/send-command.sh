#!/bin/bash

PIPE_PATH="/tmp/command-fifo"

# Send command to the pipe
echo "$1" > "$PIPE_PATH"
