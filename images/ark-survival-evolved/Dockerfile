FROM steamcmd/steamcmd:debian-bookworm

# Install rcon-cli for command interface
RUN apt-get update && \
    apt-get install --no-install-recommends -y \
        wget \
        && wget -O rcon-cli.tar.gz https://github.com/gorcon/rcon-cli/releases/download/v0.10.3/rcon-0.10.3-amd64_linux.tar.gz \
        && tar -xzf rcon-cli.tar.gz \
        && mv rcon-0.10.3-amd64_linux/rcon /usr/local/bin/rcon-cli \
        && chmod +x /usr/local/bin/rcon-cli \
        && rm -rf rcon-cli.tar.gz rcon-0.10.3-amd64_linux \
        && apt-get clean && rm -rf /var/lib/apt/lists/*

# Create directory structure
RUN mkdir -p /data/server /data/backups /data/scripts

# Download ARK server during build (faster startup)
RUN steamcmd +force_install_dir /data/server +login anonymous +app_update 376030 validate +quit

# Copy startup scripts
COPY start.sh /data/scripts/start.sh
COPY send-command.sh /data/scripts/send-command.sh
RUN chmod +x /data/scripts/start.sh /data/scripts/send-command.sh

# Set working directory
WORKDIR /data/server

# Expose ARK server ports
EXPOSE 7777/udp 27015/udp 27020/tcp

ENTRYPOINT ["/data/scripts/start.sh"]
