FROM golang:1.26.1-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go vet ./... && go test ./... && go build -o api .
FROM alpine:3.20
COPY --from=build /app/api /api
EXPOSE 8090
ENTRYPOINT ["/api"]
