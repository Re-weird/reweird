FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum* ./apps/api/
COPY packages/component-catalog/go.mod ./packages/component-catalog/
WORKDIR /src/apps/api
RUN go mod download
WORKDIR /src
COPY apps/api ./apps/api
COPY packages/component-catalog ./packages/component-catalog
WORKDIR /src/apps/api
RUN CGO_ENABLED=0 go build -o /reweird-api ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache su-exec && adduser -D -u 10001 reweird
COPY --from=builder /reweird-api /usr/local/bin/reweird-api
COPY apps/api/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["reweird-api"]
