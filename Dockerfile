FROM golang:1.21-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/vici-server .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/vici-server /app/vici-server
COPY cards.json ./cards.json
COPY templates ./templates
COPY static ./static

# The process binds all interfaces inside the container so a published port
# works. Startup refuses this non-loopback bind unless VICI_API_TOKEN is set;
# publish the port only to a protected network and provide that token.
ENV VICI_HOST=0.0.0.0 \
    VICI_PORT=8085 \
    VICI_DATA_FILE=/app/cards.json
EXPOSE 8085

CMD ["/app/vici-server"]
