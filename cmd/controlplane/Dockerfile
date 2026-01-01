FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git ca-certificates
ENV GOPROXY=https://proxy.golang.org,direct
WORKDIR /app
COPY go.mod ./
COPY pkg/ ./pkg/
COPY cmd/controlplane/ ./cmd/controlplane/
RUN go mod tidy
RUN go build -o controlplane ./cmd/controlplane

FROM alpine:3.19
COPY --from=builder /app/controlplane /controlplane
ENV PORT=8081
EXPOSE 8081
CMD ["/controlplane"]