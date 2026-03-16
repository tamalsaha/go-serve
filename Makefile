APP_NAME := go-serve
IMAGE ?= ghcr.io/appscodeci/go-serve
TAG ?= latest
K8S_MANIFEST ?= deploy/kubernetes.yaml

.PHONY: fmt build run check docker-build docker-push deploy undeploy tidy test export-branch-protection

fmt:
	gofmt -w main.go internal/server/server.go internal/tlsutil/selfsigned.go internal/check/check.go

build:
	go build -o bin/$(APP_NAME) .

run:
	go run . run --port 9443

check:
	go run . check --service go-serve --namespace default --port 9443 --scheme https --path /healthz --insecure

docker-build:
	docker build -t $(IMAGE):$(TAG) .

docker-push:
	docker push $(IMAGE):$(TAG)

deploy:
	kubectl apply -f $(K8S_MANIFEST)

undeploy:
	kubectl delete -f $(K8S_MANIFEST)

tidy:
	go mod tidy

test:
	go test ./...

export-branch-protection:
	mkdir -p .github/policy
	REPO=$$(gh repo view --json nameWithOwner -q .nameWithOwner) && \
	BRANCH=$$(gh repo view --json defaultBranchRef -q .defaultBranchRef.name) && \
	gh api -H "Accept: application/vnd.github+json" "/repos/$$REPO/branches/$$BRANCH/protection" > .github/policy/branch-protection-live.json
