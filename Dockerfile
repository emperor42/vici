FROM golang:1.20-alpine

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -o vici-server ./main.go

EXPOSE 8085

CMD ["/app/vici-server"]
