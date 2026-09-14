FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget sqlite
WORKDIR /app
COPY --from=build /out/bot /app/bot
EXPOSE 8080
ENTRYPOINT ["/app/bot"]
