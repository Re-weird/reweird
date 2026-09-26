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
RUN adduser -D -u 10001 reweird
USER reweird
COPY --from=builder /reweird-api /usr/local/bin/reweird-api
EXPOSE 8080
CMD ["reweird-api"]
