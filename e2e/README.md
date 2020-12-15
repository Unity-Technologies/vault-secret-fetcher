E2E tests for vault-secret-fetcher
---


## Usage
> All steps are need to be executed inside project root folder

1. Build an image of vault-secret-fetcher
```
image=vault-secret-fetcher:latest ./scripts/build.sh
```

2. Run E2E tests
```
E2E_VAULT_SECRET_FETCHER_IMAGE=vault-secret-fetcher:latest ./scripts/run-e2e-tests.sh
```

## Configuration
| Environment variable           | Description                | Default                                          |
| ------------------------------ | -------------------------- | ------------------------------------------------ |
| E2E_VAULT_SECRET_FETCHER_IMAGE | vault-secret-fetcher image | -                                                |
| E2E_VAULT_IMAGE                | vault image                | vault:1.6.0          |
| E2E_K8S_KIND_IMAGE             | K8s kind node image        | kindest/node:v1.20.0 |
