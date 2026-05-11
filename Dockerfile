FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags='-s -w' -o server ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /build/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
