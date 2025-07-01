FROM steamcmd/steamcmd:debian-bookworm

# Create directory structure
RUN mkdir -p /data/server /data/backups /data/scripts

# Download Valheim server during build (faster startup)
RUN steamcmd +@sSteamCmdForcePlatformType linux +force_install_dir /data/server +login anonymous +app_update 896660 validate +quit

RUN apt-get update && apt-get install -y libpulse-dev libatomic1 libc6 && rm -rf /var/lib/apt/lists/*

# Copy startup scripts
COPY start.sh /data/scripts/start.sh
RUN chmod +x /data/scripts/start.sh 

# Set working directory
WORKDIR /data/server

# Expose Valheim server ports
EXPOSE 2456/udp 2457/udp 

ENTRYPOINT ["/data/scripts/start.sh"]
