FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY cmd/controlplane/ ./cmd/controlplane/
RUN go build -o controlplane ./cmd/controlplane

FROM alpine:3.19
COPY --from=builder /app/controlplane /controlplane
ENV PORT=8081
EXPOSE 8081
CMD ["/controlplane"]

