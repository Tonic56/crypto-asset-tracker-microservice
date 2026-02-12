FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/cmd/auth ./cmd/main.go

FROM alpine:3.21

WORKDIR /app

RUN addgroup -S nonroot && adduser -S nonroot -G nonroot

COPY --from=builder /app/cmd/auth .

USER nonroot

EXPOSE 50051

CMD ["./auth"]
