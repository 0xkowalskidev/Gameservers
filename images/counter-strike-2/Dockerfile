FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        wget \
        lib32gcc-s1 \
    && wget -O rcon-cli.tar.gz https://github.com/gorcon/rcon-cli/releases/download/v0.10.3/rcon-0.10.3-amd64_linux.tar.gz \
    && tar -xzf rcon-cli.tar.gz \
    && mv rcon-0.10.3-amd64_linux/rcon /usr/local/bin/rcon-cli \
    && chmod +x /usr/local/bin/rcon-cli \
    && rm -rf rcon-cli.tar.gz rcon-0.10.3-amd64_linux \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd --create-home --shell /bin/bash steam

# Create directories
RUN mkdir -p /data/steamcmd /data/server /data/scripts /data/backups && \
    chown -R steam:steam /data

USER steam

# Download and extract SteamCMD
RUN wget -qO /data/steamcmd/steamcmd_linux.tar.gz https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz && \
    tar -xvf /data/steamcmd/steamcmd_linux.tar.gz -C /data/steamcmd && \
    rm /data/steamcmd/steamcmd_linux.tar.gz

# Download CS2 server (App ID 730)
RUN /data/steamcmd/steamcmd.sh +force_install_dir /data/server +login anonymous +app_update 730 validate +quit

WORKDIR /data/server

COPY --chown=steam:steam start.sh /data/scripts/start.sh
COPY --chown=steam:steam send-command.sh /data/scripts/send-command.sh
RUN chmod +x /data/scripts/start.sh /data/scripts/send-command.sh

EXPOSE 27015/udp 27015/tcp

ENTRYPOINT ["/data/scripts/start.sh"]
