FROM steamcmd/steamcmd:debian-bookworm

# Create directory structure
RUN mkdir -p /data/server /data/backups /data/scripts

# Download Palworld server during build (faster startup)
RUN steamcmd +force_install_dir /data/server +login anonymous +app_update 2394010 validate +quit

 

# Create steam user and set permissions
RUN useradd -m -s /bin/bash steam && chown -R steam:steam /data

USER steam

# Copy startup scripts
COPY --chown=steam:steam start.sh /data/scripts/start.sh
RUN chmod +x /data/scripts/start.sh

# Set working directory
WORKDIR /data/server

# Expose Palworld server ports
EXPOSE 8211/udp 8212/tcp

ENTRYPOINT ["/data/scripts/start.sh"]
