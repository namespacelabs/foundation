FROM node:16-alpine

WORKDIR /usr/src/app

COPY mysidecar/package*.json ./
RUN npm install

COPY mysidecar/src .

CMD [ "node", "sidecarserver.js" ]