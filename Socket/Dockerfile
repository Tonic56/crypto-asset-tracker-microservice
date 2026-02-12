FROM golang:1.25.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/cmd/socket ./cmd/main.go

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/cmd/socket .

CMD ["./socket"]
