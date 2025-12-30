FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod tidy
COPY cmd/simulator/ ./cmd/simulator/
RUN go build -o simulator ./cmd/simulator

FROM alpine:3.19
COPY --from=builder /app/simulator /simulator
CMD ["/simulator"]

