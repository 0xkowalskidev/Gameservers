FROM ubuntu:22.04

# Install Java 8, 17, and 21 for multi-version Minecraft support
# Also install curl and jq for downloading server JARs and JSON parsing
RUN apt-get update && apt-get install -y \
    curl jq \
    openjdk-8-jre-headless \
    openjdk-17-jre-headless \
    openjdk-21-jre-headless \
    && rm -rf /var/lib/apt/lists/*

# Set default Java to 21 (start.sh will override based on MC version)
ENV JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64

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
