# Stage 1: Build and extract Terraria server
FROM alpine:3.21 AS base

RUN apk add --no-cache unzip && \
    mkdir -p /terraria-server && \
    wget -O /terraria-server.zip https://terraria.org/api/download/pc-dedicated-server/terraria-server-1449.zip && \
    unzip /terraria-server.zip -d /terraria && \
    mv /terraria/1449/Linux/* /terraria-server && \
    chmod +x /terraria-server/TerrariaServer.bin.x86_64 && \
    rm /terraria-server.zip

# Stage 2: Runtime with Mono
FROM mono:6.10-slim

# Create directories for server files, backups, and scripts
RUN mkdir -p /data/server /data/backups /data/scripts

# Copy server files from build stage
COPY --from=base /terraria-server/ /data/server/

# Copy start script
COPY start.sh /data/scripts/start.sh
RUN chmod +x /data/scripts/start.sh

# Set working directory
WORKDIR /data/server

# Expose default Terraria server port
EXPOSE 7777

# Set command to run start.sh
CMD ["/data/scripts/start.sh"]
