FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY cmd/gateway/ ./cmd/gateway/
RUN go build -o gateway ./cmd/gateway

FROM alpine:3.19
COPY --from=builder /app/gateway /gateway
ENV PORT=8080
EXPOSE 8080
CMD ["/gateway"]

