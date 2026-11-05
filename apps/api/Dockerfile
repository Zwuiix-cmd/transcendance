FROM golang:1.24-alpine AS dependencies
RUN apk add --no-cache gcc musl-dev
WORKDIR /workspace/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

FROM dependencies AS test
COPY apps/api/cmd ./cmd
COPY apps/api/internal ./internal
COPY apps/api/db/schema.sql ./schema.sql
RUN go test ./...

FROM dependencies AS package
COPY apps/api/cmd ./cmd
COPY apps/api/internal ./internal
COPY apps/api/db/schema.sql ./schema.sql
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/card-arena-api ./cmd/api

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates libgcc \
    && adduser -D -H -u 10001 appuser \
    && mkdir -p /app/data \
    && chown -R appuser:appuser /app
WORKDIR /app
COPY --from=package /out/card-arena-api /app/card-arena-api
COPY --from=package /workspace/apps/api/schema.sql /app/schema.sql
RUN chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/card-arena-api"]
