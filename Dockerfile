FROM golang:1.24-alpine3.21 AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bot cmd/bot/main.go

FROM alpine:3.22

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/bot/ .

COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD [ "./bot" ]
