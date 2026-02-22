FROM golang:1.24.5-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/freedom_bitrix ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/freedom_bitrix /app/freedom_bitrix

EXPOSE 8080
ENTRYPOINT ["/app/freedom_bitrix"]
CMD ["serve-delta"]
