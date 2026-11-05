FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY tsconfig*.json vite.config.ts index.html ./
COPY src ./src
ARG VITE_API_BASE_URL=/api/v1
ARG VITE_DEBUG_AUTH=false
ARG VITE_ASSET_BASE_URL=/
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
ENV VITE_DEBUG_AUTH=${VITE_DEBUG_AUTH}
ENV VITE_ASSET_BASE_URL=${VITE_ASSET_BASE_URL}
RUN npm run build

FROM caddy:2.9-alpine
COPY --from=build /app/dist /srv
COPY Caddyfile /etc/caddy/Caddyfile
