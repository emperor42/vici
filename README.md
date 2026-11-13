# VICI

VICI is two independent pieces:

1. `vici.js` is a browser utility for non-`HttpOnly` cookies and Web Crypto encryption/decryption.
2. `main.go` is an optional local card/timeline and lore editor with a small JSON and CSV API.

The Go demo is not a WordPress-style page CMS, an authentication service, or a

## Overview

`vici` is a Go backend service that provides timeline/card management APIs, serving as the foundation for the VICI timeline editor.

## Features

- **Timeline Management**: API for creating and managing timeline cards
- **Search Functionality**: Full-text search and filtering
- **Tag Support**: Organize cards with categories
- **Health Monitoring**: Built-in health check endpoint
- **Security First**: Implements security best practices

## API Endpoints

### Timeline API
- **GET /api/cards** - List all cards with optional filtering
- **POST /api/cards** - Create a new card

### System Status
- **GET /health** - Health check endpoint

## Technology Stack

- **Backend**: Go
- **API**: RESTful JSON
- **Security**: HTTP security headers, validation
- **Deployment**: Docker ready

## Running

### Prerequisites

```bash
go 1.21+
```

### Building and Running

```bash
# Build the application
go build

# Run the application
./vici

# Or using Docker
docker build -t vici .
docker run -p 8083:8083 vici
```

### Testing

```bash
# Run tests
go test -v ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Usage

### Local Development

```bash
# Start the server
./vici

# List all cards
curl http://localhost:8083/api/cards

# Create a new card
curl -X POST -H "Content-Type: application/json" -d '{"title":"New Card","content":"Card content"}' http://localhost:8083/api/cards

# Search cards
curl "http://localhost:8083/api/cards?q=example"

# Check system health
curl http://localhost:8083/health
```

### API Examples

#### Get All Cards
```bash
curl -X GET http://localhost:8083/api/cards
```

#### Get Filtered Cards
```bash
curl -X GET "http://localhost:8083/api/cards?q=search"
```

#### Create New Card
```bash
curl -X POST -H "Content-Type: application/json" -d '{"title":"New Card","content":"Card content","tags":["example","demo"]}' http://localhost:8083/api/cards
```

#### Health Check
```bash
curl -X GET http://localhost:8083/health
```

## Deployment

### Docker Deployment

```Dockerfile
FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod go.sum ./
COPY . .

RUN go build -o vici main.go

EXPOSE 8083

CMD ["./vici"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vici
  labels:
    app: vici
spec:
  replicas: 2
  selector:
    matchLabels:
      app: vici
  template:
    metadata:
      labels:
        app: vici
    spec:
      containers:
      - name: vici
        image: vici:latest
        ports:
        - containerPort: 8083
        env:
        - name: NODE_ENV
          value: "production"
```

## Development

### Project Structure

```
.
├── main.go          # Main application
├── main_test.go     # Tests
├── go.mod          # Module configuration
└── README.md       # Documentation
```

### Features Implemented

- **Timeline Management**: Complete card lifecycle management
- **Search Functionality**: Full-text search and filtering
- **Tag Support**: Organize cards with categories
- **Security First**: Request validation and security
- **API-First**: RESTful API design
- **Developer Experience**: Comprehensive testing

## License

MIT License

## Future Enhancements

- [ ] Add card categories and tags
- [ ] Implement card attachments
- [ ] Add timeline export
- [ ] Implement user authentication
- [ ] Add collaboration features
- [ ] Implement real-time updates

## Contributing

1. Fork the repository
2. Create a feature branch
3. Implement your changes
4. Add tests
5. Submit a pull request

## Contact

For questions or suggestions, please open an issue or contact the maintainer.