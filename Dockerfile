FROM golang:1.27.3-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/noteflow ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/noteflow /app/noteflow
USER app:app
EXPOSE 3000
ENTRYPOINT ["/app/noteflow"]
CMD ["serve"]
