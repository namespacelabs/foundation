FROM node:16-alpine

WORKDIR /app

COPY package*.json ./
RUN npm install

RUN echo foobar2

COPY src ./src
