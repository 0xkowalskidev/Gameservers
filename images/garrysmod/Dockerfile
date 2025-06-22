FROM ubuntu:22.04

LABEL author="T3 Chat"
LABEL description="Garry's Mod Dedicated Server (Self-Contained)"

ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies for SteamCMD, 32-bit Source Engine, and rcon-cli
RUN apt-get update && \
    dpkg --add-architecture i386 && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    lib32gcc-s1 \
    libstdc++6:i386 \
    libsdl2-2.0-0:i386 \
    && wget -O rcon-cli.tar.gz https://github.com/gorcon/rcon-cli/releases/download/v0.10.3/rcon-0.10.3-amd64_linux.tar.gz \
    && tar -xzf rcon-cli.tar.gz \
    && mv rcon-0.10.3-amd64_linux/rcon /usr/local/bin/rcon-cli \
    && chmod +x /usr/local/bin/rcon-cli \
    && rm -rf rcon-cli.tar.gz rcon-0.10.3-amd64_linux \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Create a non-root user for security
RUN useradd --create-home --shell /bin/bash steam

# Create directories as requested
RUN mkdir -p /data/steamcmd /data/server /data/scripts /data/backups && \
    chown -R steam:steam /data

# Switch to the steam user
USER steam

# Download and extract SteamCMD
RUN wget -qO /data/steamcmd/steamcmd_linux.tar.gz https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz && \
    tar -xvf /data/steamcmd/steamcmd_linux.tar.gz -C /data/steamcmd && \
    rm /data/steamcmd/steamcmd_linux.tar.gz

# --- Download GMod Server Files During Build ---
RUN /data/steamcmd/steamcmd.sh \
  +force_install_dir /data/server \
  +login anonymous \
  +app_update 4020 validate \
  +quit

# Set the working directory
WORKDIR /data/server

# Copy the start and send-command scripts into the container and make them executable
COPY --chown=steam:steam start.sh /data/scripts/start.sh
COPY --chown=steam:steam send-command.sh /data/scripts/send-command.sh
COPY --chown=steam:steam stop.sh /data/scripts/stop.sh
RUN chmod +x /data/scripts/start.sh /data/scripts/send-command.sh /data/scripts/stop.sh

# Expose the default Garry's Mod server ports
EXPOSE 27015/udp
EXPOSE 27015/tcp

# Set the entrypoint to our start script
ENTRYPOINT ["/data/scripts/start.sh"]
