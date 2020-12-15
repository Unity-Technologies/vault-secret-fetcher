FROM golang:1.15-alpine

WORKDIR /src

RUN apk --no-cache add curl docker-cli gcc && \
    curl -L -o /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.19.0/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl
COPY . .

