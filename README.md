# Vault Secret Fletcher

A utility for automatically fetching secrets from Vault in IE infra. Written in Go and compiled statically to ensure no external dependencies are needed, especially in `scratch` and `distroless` images. This tool's target audience are teams looking to have maximum compatilibty with minimal developer effort in a large infrastructure already deployed in Kubernetes.

## How does it work?
1. Developer writes secrets and configures their services as described in this README.
2. Cluster is configured by operators/admins with something like `bootstrap-gke.sh`
3. Kubernetes mounts `vault-secret-fetcher` volume to the service's container.
4. `secret-fetcher` binary is executed and the service's entry point is passed as its first argument.
5. Vault Secret Fetcher (VSF) authenticates to Vault using Kubernetes as an authentication backend.
6. VSF reads secret paths from the service's env vars.
7. VSF fetches the secrets and replaces the paths with the actual secret's value **in VFS own environment**.
8. VSF replaces its own process with the service's entry point.
9. Service's entry point is now running and has access to the secrets.

**Note: Due to this design, nothing but the service's entry point has access to the secrets, i.e. dumping environment variables through Kubernetes commands will not display the secrets' values.**

## How do I build it?
Build the image with `scripts/build.sh` to ensure the resulting binary is fully statically-linked.

## Usage

After boostrapping the cluster along the lines of `bootstrap-gke.sh`, the service needs to be configured as follows:

### Format 1

#### Setup

- Secrets should be set in Vault in this manner:

    ```
    vault write secret/<YOUR PATH> secret="..."
    ```
- A proposed pattern is:

    ```
    vault write secret/<env>/<namespace>/<service_name>/<KEY_NAME> secret=<VALUE_OF_SECRET>
    ```

- You *must* use 'secret' as the key in <KEY_NAME> or the fetcher won't be able to load it.

#### Kubernetes

Use this in your Kubernetes manifest:

```
spec:
  restartPolicy: Always
  initContainers:
    - name: init-vault-secret-fetcher
      image: registry/vault-secret-fetcher:master
      command:
        ["sh", "-c", "cp /root/vault-secret-fetcher /opt/secret-fetcher"]
      imagePullPolicy: IfNotPresent
      volumeMounts:
        - name: init-vault-secret-fetcher-volume
          mountPath: /opt/secret-fetcher
  containers:
    - name: my-service
      image: gcr.io/my-project/my-service
      command:
        - "/opt/secret-fetcher/vault-secret-fetcher"
      args:
        - "python3"
        - "-m"
        - "http.server"
      env:
        - name: VAULT_ADDR
          value: "https://vault.corp"
        - name: KUBERNETES_CLUSTER
          valueFrom:
            configMapKeyRef:
              name: kubernetes-cluster
              key: kubernetes-cluster
        - name: VAULT_ROLE
          value: default-vault-sa
        - name: KUBERNETES_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: VAULT_SERVICE_ACCOUNT_JWT
          valueFrom:
              secretKeyRef:
                  name: default-vault-sa-secret
                  key: token
        - name: TEST_VAULT_ENVVAR
          value: "{{vault-secret secret/dev/sre/minikube/TEST_VAULT_ENVVAR}}"
      volumeMounts:
        - mountPath: /opt/secret-fetcher
          name: init-vault-secret-fetcher-volume
  volumes:
    - name: init-vault-secret-fetcher-volume
      emptyDir: {}
```

### Format 2

This format allows for using keys other than 'secret' when setting your values as well as having multiple keys in a secret.

#### Setup

- Secrets can be set in Vault with whatever key names you want.

#### Kubernetes

To use this in your Kubernetes manifest:

```
spec:
  restartPolicy: Always
  initContainers:
    - name: init-vault-secret-fetcher
      image: registry2.applifier.info:5005/vault-secret-fetcher:master
      command:
        ["sh", "-c", "cp /root/vault-secret-fetcher /opt/secret-fetcher"]
      imagePullPolicy: IfNotPresent
      volumeMounts:
        - name: init-vault-secret-fetcher-volume
          mountPath: /opt/secret-fetcher
      env:
        name: FETCHER_SECRET_FORMAT
        value: 2
  containers:
    - name: my-service
      image: gcr.io/my-project/my-service
      command:
        - "/opt/secret-fetcher/vault-secret-fetcher"
      args:
        - "python3"
        - "-m"
        - "http.server"
      env:
        - name: VAULT_ADDR
          value: "https://vault.corp"
        - name: KUBERNETES_CLUSTER
          valueFrom:
            configMapKeyRef:
              name: kubernetes-cluster
              key: kubernetes-cluster
        - name: VAULT_ROLE
          value: default-vault-sa
        - name: KUBERNETES_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: VAULT_SERVICE_ACCOUNT_JWT
          valueFrom:
              secretKeyRef:
                  name: default-vault-sa-secret
                  key: token
        - name: TEST_VAULT_ENVVAR
          value: "VAULTSECRET::{"path":"secret/sre/dev/TEST", "key":"TEST"}"
      volumeMounts:
        - mountPath: /opt/secret-fetcher
          name: init-vault-secret-fetcher-volume
  volumes:
    - name: init-vault-secret-fetcher-volume
      emptyDir: {}
```

Regardless of which format you choose the logs in the container should look like something this if everything is working:

```
2019/01/25 23:18:19 INFO: Vault Secret Fetcher started (revision: d53247a)
2019/01/25 23:18:19 INFO: PrepareHTTPClient() failed to get the system's cert store; using embedded cert
2019/01/25 23:18:19 INFO: GetenvSafe() failed to load 'VAULT_KV_VERSION'. It's either empty or not set
2019/01/25 23:18:22 INFO: Secrets fetched: 1/1
# your service should start at this point
```

## Debugging

If a secret isn't being set the way you expect you can turn on debug logging in the fetcher container:

```
  initContainers:
    - name: init-vault-secret-fetcher
      image: registry/vault-secret-fetcher:master
      command:
        ["sh", "-c", "cp /root/vault-secret-fetcher /opt/secret-fetcher"]
      imagePullPolicy: IfNotPresent
      volumeMounts:
        - name: init-vault-secret-fetcher-volume
          mountPath: /opt/secret-fetcher
      env:
        name: FETCHER_DEBUG
        value: true
```

## Caveats

- Access is restricted to the namespace level. All services in the same namespace have access to all secrets in the namespace.

- This assumes a proper Vault policy is in place for developer. Ideally, you want something that can be written but no read. An example policy is provided in `example-developer-policy.hcl`.

- This assumes proper logging and log monitoring is in place for Vault.

## Credits

Vault Secret Fetcher was created in late 2017 by Unity Secret Team and the Infrastructure Engineering team. More than half a dozen people contributed to it.
