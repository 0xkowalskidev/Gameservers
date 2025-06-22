FROM ubuntu:22.04

LABEL author="T3 Chat"
LABEL description="Garry's Mod Dedicated Server (Self-Contained)"

ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies for SteamCMD and the 32-bit Source Engine
RUN apt-get update && \
    dpkg --add-architecture i386 && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    lib32gcc-s1 \
    libstdc++6:i386 \
    libsdl2-2.0-0:i386 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Create a non-root user for security
RUN useradd --create-home --shell /bin/bash steam

# Create directories as requested
RUN mkdir -p /data/steamcmd /data/server /data/scripts && \
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

# Copy the start script into the container and make it executable
COPY --chown=steam:steam start.sh /data/scripts/start.sh
RUN chmod +x /data/scripts/start.sh

# Expose the default Garry's Mod server ports
EXPOSE 27015/udp
EXPOSE 27015/tcp

# Set the entrypoint to our start script
ENTRYPOINT ["/data/scripts/start.sh"]
