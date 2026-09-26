FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /reweird-api ./cmd/server

FROM alpine:3.21
RUN adduser -D -u 10001 reweird
USER reweird
COPY --from=builder /reweird-api /usr/local/bin/reweird-api
EXPOSE 8080
CMD ["reweird-api"]
