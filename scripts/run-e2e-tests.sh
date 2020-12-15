#!/bin/bash
set -e

E2E_VAULT_IMAGE_DEFAULT=vault:1.6.0
E2E_K8S_KIND_IMAGE_DEFAULT=node:v1.20.0


DOCKER_NETWORK_NAME="vault-secret-fetcher-e2e-${identifier:=$(date +%s)}"


E2E_IMAGE_NAME="vault-secret-fetcher-e2e:${revision:=latest}"
E2E_VAULT_IMAGE="${E2E_VAULT_IMAGE:=$E2E_VAULT_IMAGE_DEFAULT}"
E2E_VAULT_SECRET_FETCHER_IMAGE=${E2E_VAULT_SECRET_FETCHER_IMAGE:=${image}}
E2E_K8S_KIND_IMAGE=${E2E_K8S_KIND_IMAGE:=$E2E_K8S_KIND_IMAGE_DEFAULT}

if [ -z "$E2E_VAULT_SECRET_FETCHER_IMAGE" ]; then
  echo "Environment variable E2E_VAULT_SECRET_FETCHER_IMAGE must be set"
  exit 1
fi

pull_image_if_not_exist () {
  image=$1
  if [[ "$(docker images -q "$image" 2> /dev/null)" != "" ]]; then
    echo "Image $image is already downloaded..."
    return
  fi
  echo "Pulling image $image..."
  docker pull "$image"
}

cleanup() {
  echo "Clean up resources..."
  docker network rm "$DOCKER_NETWORK_NAME";
  docker image rm "$E2E_IMAGE_NAME";
}


trap cleanup ERR EXIT

pull_image_if_not_exist $E2E_VAULT_IMAGE
pull_image_if_not_exist $E2E_K8S_KIND_IMAGE
pull_image_if_not_exist $E2E_VAULT_SECRET_FETCHER_IMAGE

docker build -f e2e/Dockerfile -t "$E2E_IMAGE_NAME" e2e
docker network create "$DOCKER_NETWORK_NAME" || true

docker run --rm -e CGO_ENABLED=0 \
              -e E2E_VAULT_SECRET_FETCHER_IMAGE="$E2E_VAULT_SECRET_FETCHER_IMAGE" \
              -e E2E_VAULT_IMAGE="$E2E_VAULT_IMAGE" \
              -e E2E_K8S_KIND_IMAGE="$E2E_K8S_KIND_IMAGE" \
              -e KIND_EXPERIMENTAL_DOCKER_NETWORK="$DOCKER_NETWORK_NAME" \
              -v /var/run/docker.sock:/var/run/docker.sock \
              --network "$DOCKER_NETWORK_NAME" \
              $E2E_IMAGE_NAME go test
