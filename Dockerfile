FROM golang:1.15.2 as builder
WORKDIR /go/src/vault-secret-fetcher
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -ldflags "-X main.build=$(git rev-parse --short HEAD) -extldflags '-static' -s -w" -v -o ./vault-secret-fetcher

FROM alpine:latest
WORKDIR /root/

COPY --from=builder /go/src/vault-secret-fetcher .

# Make the binary readable for any user. Otherwise pods that are not running a non-root security context are unable to initialize.
RUN chmod +x /root
RUN chmod 755 /root/vault-secret-fetcher
