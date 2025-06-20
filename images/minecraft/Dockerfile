FROM eclipse-temurin:21-jre

WORKDIR /minecraft

# Download vanilla Minecraft server
RUN wget -O server.jar https://piston-data.mojang.com/v1/objects/4707d00eb834b446575d89a61a11b5d548d8c001/server.jar

# Accept EULA
RUN echo "eula=true" > eula.txt

# Copy startup script
COPY start.sh start.sh
RUN chmod +x start.sh

EXPOSE 25565

CMD ["./start.sh"]