FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git ca-certificates
ENV GOPROXY=https://proxy.golang.org,direct
WORKDIR /app
COPY go.mod ./
COPY pkg/ ./pkg/
COPY cmd/gateway/ ./cmd/gateway/
RUN go mod tidy
RUN go build -o gateway ./cmd/gateway

FROM alpine:3.19
COPY --from=builder /app/gateway /gateway
ENV PORT=8080
EXPOSE 8080
CMD ["/gateway"]