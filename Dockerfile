FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY main.go painel-fornecedor.html ./
RUN go mod tidy && go build -o api .
FROM alpine:3.20
COPY --from=build /app/api /api
EXPOSE 8090
ENTRYPOINT ["/api"]
