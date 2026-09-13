# URL Shortener

A simple backend service that shortens long URLs into short codes. Built in Go to learn HTTP basics.

Status: in development

## Features

- Generate a short code for a URL
- Redirect from the short code to the original URL

## Tech Stack

Go, net/http, in-memory storage

## API

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| POST | /shorten | Accepts a URL, returns a short code |
| GET | /:code | Redirects to the original URL |

## Running Locally

```bash
git clone https://github.com/PuggyPotato/url-shortener.git
cd url-shortener
go run main.go
```

Server runs on localhost:8080 by default.

### Example usage

```bash
curl -X POST localhost:8080/shorten -d '{"url": "https://example.com/some/long/path"}'

curl -L localhost:8080/abc123
```