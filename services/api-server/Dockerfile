FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/migrate ./cmd/migrate

FROM alpine:3.20

RUN adduser -D -H -u 10001 appuser
USER appuser

WORKDIR /app
COPY --from=build /out/api-server /app/api-server
COPY --from=build /out/migrate /app/migrate
COPY migrations /app/migrations

EXPOSE 8080

ENTRYPOINT ["/app/api-server"]
