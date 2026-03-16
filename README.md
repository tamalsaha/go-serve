# go-serve

Small Golang CLI with two commands:

- `run`: starts an HTTPS server on port `9443` with an in-memory self-signed certificate.
- `check`: performs an HTTP(S) request to a Kubernetes Service in the same cluster.

## Commands

### Run server

```bash
go run . run
```

Server defaults:

- Address: `0.0.0.0:9443`
- Endpoints:
	- `GET /`
	- `GET /healthz`

Optional flags:

```bash
go run . run --port 9443 --host 0.0.0.0
```

### Check service connectivity

From inside a Kubernetes cluster, this command can resolve services using cluster DNS.

```bash
go run . check --service go-serve --namespace default --port 9443 --scheme https --path /healthz --insecure
```

Common flags:

- `--service`: service name (required)
- `--namespace`: service namespace (default: `default`)
- `--port`: service port (default: `9443`)
- `--scheme`: `http` or `https` (default: `https`)
- `--path`: request path (default: `/healthz`)
- `--insecure`: skip TLS verification for self-signed certs (default: `true`)
- `--timeout`: request timeout (default: `5s`)

## Build Docker Image

```bash
docker build -t ghcr.io/appscodeci/go-serve:latest .
```

Or with Make:

```bash
make docker-build TAG=latest
```

## Kubernetes Deployment

Apply:

```bash
kubectl apply -f deploy/kubernetes.yaml
```

This manifest includes:

- ServiceAccount `go-serve`
- Deployment `go-serve`
- Service `go-serve`

## Makefile Targets

```bash
make fmt
make build
make run
make check
make docker-build TAG=latest
make docker-push TAG=latest
make deploy
make undeploy
make test
```

## GitHub Actions Release

A tag push matching `v*` triggers image build and push to:

```text
ghcr.io/appscodeci/go-serve
```

Workflow file:

- `.github/workflows/release.yaml`

## GitHub Actions CI

Pull requests and pushes to `main` run tests and build the multi-arch Docker image without pushing:

- `.github/workflows/ci.yaml`

## Branch Protection

Recommended branch protection setup and required checks are documented in:

- `.github/branch-protection.md`

Export current live branch protection policy to a tracked JSON file:

```bash
make export-branch-protection
```

Output file:

- `.github/policy/branch-protection-live.json`