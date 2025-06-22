FROM eclipse-temurin:21-jre

# Create directory structure
RUN mkdir -p /data/server /data/backups /data/scripts
WORKDIR /data/server

# Download vanilla Minecraft server
RUN wget -O server.jar https://piston-data.mojang.com/v1/objects/4707d00eb834b446575d89a61a11b5d548d8c001/server.jar

# Copy startup script and command script
COPY server.properties /data/server
COPY start.sh /data/scripts/start.sh
COPY send-command.sh /data/scripts/send-command.sh
COPY stop.sh /data/scripts/stop.sh
RUN chmod +x /data/scripts/start.sh /data/scripts/send-command.sh /data/scripts/stop.sh

EXPOSE 25565

CMD ["/data/scripts/start.sh"]
