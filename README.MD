<p align="center">
  <img src="assets/volare.png" alt="Volare logo" width="462" />
</p>

<p align="center">
  <a href="https://github.com/AdamShannag/volare/actions/workflows/build.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/AdamShannag/volare/build.yml?branch=master&label=CI%2FCD&logo=github" alt="CI/CD">
  </a>
  <a href="https://github.com/AdamShannag/volare/blob/master/go.mod">
    <img src="https://img.shields.io/github/go-mod/go-version/AdamShannag/volare?logo=go" alt="Go Version">
  </a>
  <a href="https://github.com/AdamShannag/volare/actions/workflows/build.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/AdamShannag/volare/build.yml?branch=master&label=Trivy%20Security%20Scan&logo=aquasec" alt="Trivy Security Scan">
  </a>
  <a href="https://github.com/AdamShannag/volare/releases/latest">
    <img src="https://img.shields.io/github/v/release/AdamShannag/volare?sort=semver" alt="Latest Release">
  </a>
  <a href="https://github.com/AdamShannag/volare/blob/master/LICENSE">
    <img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT">
  </a>
</p>

## About Volare

**Volare** is a Kubernetes volume populator that populates Persistent Volume Claims (PVCs) from multiple external
sources concurrently.

## Supported Source Types

- **HTTP** – Direct file URLs
- **GitHub**
- **GitLab**
- **S3** – S3 compatible providers

## Getting Started

### 1. Install VolumeDataSource Validator (Upstream Requirement)

Volare relies on the upstream [CSI Volume Populator](https://github.com/kubernetes-csi/volume-data-source-validator).
Install the required CRDs and RBAC by running:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/volume-data-source-validator/v1.0.1/client/config/crd/populator.storage.k8s.io_volumepopulators.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/volume-data-source-validator/v1.0.1/deploy/kubernetes/rbac-data-source-validator.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/volume-data-source-validator/v1.0.1/deploy/kubernetes/setup-data-source-validator.yaml
```

### 2. Deploy Volare CRDs and Populators

Apply the custom resource definitions and Volare volume populators:

```bash
kubectl apply -f https://raw.githubusercontent.com/AdamShannag/volare/master/manifests/1.volarepopulators-crd.yaml
kubectl apply -f https://raw.githubusercontent.com/AdamShannag/volare/master/manifests/2.volumepopulator.yaml
```

### 3. Deploy Volare Controller & Example PVC

Start the Volare controller and create a sample PVC using a VolarePopulator:

```bash
kubectl apply -f https://raw.githubusercontent.com/AdamShannag/volare/master/manifests/3.volarepopulator.yaml
kubectl apply -f https://raw.githubusercontent.com/AdamShannag/volare/master/manifests/4.volare-controller.yaml
kubectl apply -f https://raw.githubusercontent.com/AdamShannag/volare/master/manifests/5.pvc.yaml
```

### 4. Verify the Mounted Data

Deploy a checker pod and inspect the mounted data:

```bash
kubectl apply -f https://raw.githubusercontent.com/AdamShannag/volare/master/manifests/6.checker.yaml
```

Then open a shell inside the pod:

```bash
kubectl exec -it nginx -- bash
cd /mnt/checker
ls
```

## Sources Configuration Reference

A detailed overview of all supported source types (`http`, `gitlab`, `github`, `s3`), their available configuration
options, and practical usage examples.

> **Note on Environment Variable**
> For sensitive fields like tokens and credentials, values can be loaded from environment variables.
> If the value matches the name of an environment variable, it will be substituted; otherwise, the literal value will be
> used as-is.

### Fields that support environment variable:

| Source Type | Field                 |
|-------------|-----------------------|
| `http`      | Headers (values only) |
| `gitlab`    | `token`               |
| `github`    | `token`               |
| `s3`        | `accessKeyId`         |
| `s3`        | `secretAccessKey`     |
| `s3`        | `sessionToken`        |

> Example:
> If you set `token: GITLAB_TOKEN` in your config and your environment has `GITLAB_TOKEN=abcd1234`, it will use
`abcd1234`.

> These environment variables can be injected directly into the controller deployment like this:

```yaml
env:
  - name: GITLAB_TOKEN
    value: secret-token
```

> Full example snippet from a Kubernetes controller deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller
  namespace: controller
spec:
  replicas: 1
  selector:
    matchLabels:
      app: controller
  template:
    metadata:
      labels:
        app: controller
    spec:
      serviceAccountName: vp-account
      containers:
        - name: volarepopulator
          image: ghcr.io/adamshannag/volare:v0.2.0
          args:
            - "--mode=controller"
            - "--image=ghcr.io/adamshannag/volare:v0.2.0"
            - "--namespace=controller"
            - "--mountpath=/mnt/checker"
          env:
            - name: GITLAB_TOKEN
              value: secret-token
```

### Common Required Fields (All Types)

| Field        | Type   | Required | Description                                          |
|--------------|--------|----------|------------------------------------------------------|
| `type`       | string | ✅        | One of: `http`, `gitlab`, `github`, `s3`             |
| `targetPath` | string | ✅        | Relative path under `mountPath` to store the file(s) |

### HTTP Source

| Field          | Type   | Required | Description                                    |
|----------------|--------|----------|------------------------------------------------|
| `http.uri`     | string | ✅        | URI of the file to download (must be full URL) |
| `http.headers` | object | ❌        | Optional HTTP headers (e.g., auth)             |

#### Example

```yaml
- type: http
  targetPath: docs
  http:
    uri: https://example.com/readme.md
```

### GitLab Source

| Field            | Type      | Required | Description                                                                                                                                    |
|------------------|-----------|----------|------------------------------------------------------------------------------------------------------------------------------------------------|
| `gitlab.host`    | string    | ✅        | GitLab host (e.g., `https://gitlab.com`)                                                                                                       |
| `gitlab.project` | string    | ✅        | Full project path (e.g., `group/my-project`)                                                                                                   |
| `gitlab.ref`     | string    | ✅        | Git reference (branch/tag/commit)                                                                                                              |
| `gitlab.paths`   | string\[] | ✅        | List of file or directory keys to download. Keys ending with / will create the corresponding directory; otherwise only contents are extracted. |
| `gitlab.token`   | string    | ❌        | Required if private repo                                                                                                                       |
| `gitlab.workers` | integer   | ❌        | Optional, default is 2                                                                                                                         |

#### Example

```yaml
- type: gitlab
  targetPath: integrations
  gitlab:
    host: https://gitlab.com
    project: group/my-project
    ref: master
    paths:
      - integration/testutils/ # will create testutils directory and its content
      - integration/testutils # won't create testutils directory, just its content
      - integration/testutils/db.go
    token: secret-token
    workers: 2
```

### GitHub Source

| Field            | Type      | Required | Description                                                                                                                                    |
|------------------|-----------|----------|------------------------------------------------------------------------------------------------------------------------------------------------|
| `github.owner`   | string    | ✅        | GitHub repository owner                                                                                                                        |
| `github.repo`    | string    | ✅        | Repository name                                                                                                                                |
| `github.ref`     | string    | ✅        | Git reference (branch/tag/commit)                                                                                                              |
| `github.paths`   | string\[] | ✅        | List of file or directory keys to download. Keys ending with / will create the corresponding directory; otherwise only contents are extracted. |
| `github.token`   | string    | ❌        | Required if private repo                                                                                                                       |
| `github.workers` | integer   | ❌        | Optional, default is 2                                                                                                                         |

#### Example

```yaml
- type: github
  targetPath: /github
  github:
    owner: kubernetes-csi
    repo: lib-volume-populator
    ref: master
    paths:
      - example/ # will create example directory and its content
      - example # won't create example directory, just its content
    workers: 2
```

### S3 Source

| Field                | Type      | Required | Description                                                                                                                                    |
|----------------------|-----------|----------|------------------------------------------------------------------------------------------------------------------------------------------------|
| `s3.endpoint`        | string    | ✅        | S3-compatible service endpoint                                                                                                                 |
| `s3.secure`          | boolean   | ❌        | Use HTTPS (default: false)                                                                                                                     |
| `s3.bucket`          | string    | ✅        | Name of the bucket                                                                                                                             |
| `s3.paths`           | string\[] | ✅        | List of file or directory keys to download. Keys ending with / will create the corresponding directory; otherwise only contents are extracted. |
| `s3.region`          | string    | ❌        | Region (optional for some services)                                                                                                            |
| `s3.accessKeyId`     | string    | ✅        | Access key ID                                                                                                                                  |
| `s3.secretAccessKey` | string    | ✅        | Secret access key                                                                                                                              |
| `s3.sessionToken`    | string    | ❌        | Temporary token (if using session auth)                                                                                                        |
| `s3.workers`         | integer   | ❌        | Optional, default is 2                                                                                                                         |

#### Example

```yaml
- type: s3
  targetPath: /s3
  s3:
    endpoint: fra1.digitaloceanspaces.com
    secure: true
    bucket: bucket-name
    region: us-east-1
    accessKeyId: access-key-id
    secretAccessKey: secret
    paths:
      - backups/ # will create backup directory and its content
      - backups # won't create backup directory, just its content
      - images/image1.png
    workers: 3
```

### Global Options

| Field     | Type    | Required | Description                                             |
|-----------|---------|----------|---------------------------------------------------------|
| `workers` | integer | ❌        | Global number of concurrent source workers (default: 2) |

### Example `VolarePopulator`

```yaml
apiVersion: k8s.volare.dev/v1alpha1
kind: VolarePopulator
metadata:
  name: volare-populator
spec:
  sources:
    - type: github
      targetPath: /github
      github:
        owner: kubernetes-csi
        repo: lib-volume-populator
        ref: master
        paths:
          - example
        # public repo       
        # token: GITHUB_TOKEN # Read from env if set, otherwise used as a raw string
    - type: s3
      targetPath: /s3
      s3:
        endpoint: s3.example.com
        bucket: data-bucket
        region: us-east-1
        accessKeyId: AWS_ACCESS_KEY # Read from env if set, otherwise used as a raw string
        secretAccessKey: AWS_SECRET_KEY # Read from env if set, otherwise used as a raw string
        secure: true
        paths:
          - data/
  workers: 4
```

## License

[MIT](LICENSE)

## Contribution

If you have any questions or want to contribute, feel free to open an issue or PR.
