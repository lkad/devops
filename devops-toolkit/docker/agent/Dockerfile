FROM node:18-alpine

WORKDIR /app

COPY package*.json ./
RUN npm install --only=production 2>/dev/null || true

COPY devices/agent.js ./
COPY config/config.js ./

EXPOSE 8081

CMD ["node", "devices/agent.js"]
