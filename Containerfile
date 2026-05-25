FROM golang:1.26-alpine AS builder

ARG VERSION=dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /ddg-mcp .

FROM scratch
COPY --from=builder /ddg-mcp /ddg-mcp
ENTRYPOINT ["/ddg-mcp"]