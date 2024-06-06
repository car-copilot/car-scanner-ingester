FROM golang:1.21-alpine as build

WORKDIR /build

COPY go.mod .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GARCH=amd64 go build -o server main.go

FROM alpine:3

WORKDIR /app

COPY --from=build /build/server /app/server

COPY config.yaml.example /app/config.yaml

RUN chmod +x /app/server

EXPOSE 8080
CMD ["/app/server"]
