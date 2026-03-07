FROM golang:1.22 AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/gpanel-agent main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates bash curl
COPY --from=builder /out/gpanel-agent /usr/local/bin/gpanel-agent
ENTRYPOINT ["/usr/local/bin/gpanel-agent"]
