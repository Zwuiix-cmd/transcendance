FROM golang:1.24-alpine AS build

WORKDIR /src

COPY apps/cdn/go.mod ./
COPY apps/cdn/cmd ./cmd
COPY apps/cdn/internal ./internal
COPY scripts/avatar_generator.py ./scripts/avatar_generator.py

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/cdn ./cmd/cdn

FROM alpine:3.22

RUN apk add --no-cache python3 libwebp-tools libavif-apps \
    && adduser -D -H -u 10001 appuser \
    && mkdir -p /data/files \
    && chown -R appuser:appuser /data

COPY --from=build /out/cdn /cdn
COPY --from=build /src/scripts/avatar_generator.py /usr/local/bin/avatar_generator.py

USER appuser
WORKDIR /data
ENV AVATAR_GENERATOR_SCRIPT=/usr/local/bin/avatar_generator.py

EXPOSE 8091
ENTRYPOINT ["/cdn"]
