# Microservices System

This project implements a microservices system with three services:

1. **APIGateway** - Entry point for all client requests (port 8080)
2. **CommentService** - Manages comments in a database (port 8081)
3. **CensorService** - Checks comments for prohibited content (port 8082)

## Architecture

The system implements the following flow for comment creation:
1. Client sends comment to APIGateway
2. APIGateway validates text with CensorService
3. If valid, APIGateway saves to CommentService
4. If invalid, APIGateway returns error to client

## Running the System

1. Start CensorService:
```bash
cd censor-service
go run main.go
```

2. Start CommentService:
```bash
cd comment-service
go run main.go
```

3. Start APIGateway:
```bash
cd api-gateway
go run main.go
```

## API Endpoints

### APIGateway (localhost:8080)
- `GET /news` - List news with pagination
- `GET /news?page=2` - Second page of news
- `GET /news?s={query}` - Search news by title
- `GET /news/{id}` - Detailed news with comments
- `POST /comment` - Add a comment (requires censorship check)

### CommentService (localhost:8081)
- `POST /comments` - Create a comment
- `GET /comments?news_id={id}` - Get comments for a news item

### CensorService (localhost:8082)
- `POST /check` - Check text for prohibited content

## Features

- Cross-cutting request ID tracking
- Request/response logging
- Prohibited word filtering (qwerty, йцукен, zxvbnm)
- Pagination for news listing
- Comment hierarchy support