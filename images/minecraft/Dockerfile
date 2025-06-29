FROM eclipse-temurin:21-jre

# Install curl and jq for downloading server JARs and JSON parsing
RUN apt-get update && apt-get install -y curl jq && rm -rf /var/lib/apt/lists/*

# Create directory structure
RUN mkdir -p /data/server /data/backups /data/scripts
WORKDIR /data/server

# Copy startup script and command script
COPY server.properties /data/server
COPY start.sh /data/scripts/start.sh
COPY send-command.sh /data/scripts/send-command.sh
RUN chmod +x /data/scripts/start.sh /data/scripts/send-command.sh

EXPOSE 25565

CMD ["/data/scripts/start.sh"]
