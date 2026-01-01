FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git ca-certificates
ENV GOPROXY=https://proxy.golang.org,direct
WORKDIR /app
COPY go.mod ./
COPY pkg/ ./pkg/
COPY cmd/simulator/ ./cmd/simulator/
RUN go mod tidy
RUN go build -o simulator ./cmd/simulator

FROM alpine:3.19
COPY --from=builder /app/simulator /simulator
CMD ["/simulator"]

