FROM ubuntu:22.04

# Install dependencies for Rust server (SteamCMD + 32-bit libraries + rcon-cli)
RUN dpkg --add-architecture i386 && \
    apt-get update && \
    apt-get install -y \
        lib32gcc-s1 \
        lib32stdc++6 \
        libsdl2-2.0-0:i386 \
        curl \
        wget \
        ca-certificates \
        && wget -O rcon-cli.tar.gz https://github.com/gorcon/rcon-cli/releases/download/v0.10.3/rcon-0.10.3-amd64_linux.tar.gz \
        && tar -xzf rcon-cli.tar.gz \
        && mv rcon-0.10.3-amd64_linux/rcon /usr/local/bin/rcon-cli \
        && chmod +x /usr/local/bin/rcon-cli \
        && rm -rf rcon-cli.tar.gz rcon-0.10.3-amd64_linux \
        && rm -rf /var/lib/apt/lists/*

# Create steam user for security
RUN useradd -m steam

# Create directory structure
RUN mkdir -p /data/server /data/backups /data/scripts /data/steamcmd
RUN chown -R steam:steam /data

# Install SteamCMD
USER steam
WORKDIR /data/steamcmd
RUN curl -sqL "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz" | tar zxvf -

# Download Rust server during build (faster startup)
RUN ./steamcmd.sh +force_install_dir /data/server +login anonymous +app_update 258550 validate +quit

USER root

# Copy startup scripts
COPY start.sh /data/scripts/start.sh
COPY send-command.sh /data/scripts/send-command.sh
RUN chmod +x /data/scripts/start.sh /data/scripts/send-command.sh
RUN chown steam:steam /data/scripts/start.sh /data/scripts/send-command.sh

# Set working directory
WORKDIR /data/server
USER steam

# Expose Rust server ports
# 28015/tcp - Game port
# 28016/tcp - RCON port 
EXPOSE 28015 28016

CMD ["/data/scripts/start.sh"]
