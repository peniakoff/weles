# Image name
IMAGE ?= weles
# Go toolchain (override if needed)
GO ?= go
# Lambda output binary name required by provided.al2023
LAMBDA_BIN := bootstrap

.PHONY: tidy build test test-race lint docker-build lambda-build sam-build sam-validate run clean build-FeedbackFunction deploycheck

tidy:
	$(GO) mod tidy

build:
	$(GO) build -buildvcs=false -o bin/server ./cmd/server

run:
	$(GO) run ./cmd/server

test:
	$(GO) test ./... -count=1

deploycheck:
	@test -n "$(APPS)" || (echo "usage: make deploycheck APPS=config/apps.yaml IDENTITIES=example.com [REGION=… ACCOUNT=… PRINT_ARNS=1]" >&2; exit 1)
	@test -n "$(IDENTITIES)" || (echo "usage: make deploycheck APPS=config/apps.yaml IDENTITIES=example.com [REGION=… ACCOUNT=… PRINT_ARNS=1]" >&2; exit 1)
	$(GO) run ./cmd/deploycheck -apps "$(APPS)" -identities "$(IDENTITIES)" $(if $(REGION),-region $(REGION),) $(if $(ACCOUNT),-account $(ACCOUNT),) $(if $(PRINT_ARNS),-print-arns,)

test-race:
	$(GO) test ./... -count=1 -race

lint:
	golangci-lint run ./...

lambda-build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build -buildvcs=false -tags lambda.norpc -ldflags="-s -w" -o $(LAMBDA_BIN) ./cmd/lambda

# SAM Makefile build target (Metadata.BuildMethod: makefile)
build-FeedbackFunction:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build -buildvcs=false -tags lambda.norpc -ldflags="-s -w" -o $(ARTIFACTS_DIR)/bootstrap ./cmd/lambda

docker-build:
	docker build -t $(IMAGE):latest .

sam-validate:
	sam validate --lint

sam-build:
	sam build

clean:
	rm -rf bin/ $(LAMBDA_BIN) .aws-sam/
