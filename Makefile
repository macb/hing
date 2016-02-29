GITCOMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)
REGISTRY ?= $(hub.docker.com/macb)

all: build

publish: build release clean

build:
	@echo "==> Building the project"
	@env GOOS=linux GOARCH=amd64 go build -o hing

release:
	@echo "==> Building the docker image"
	@docker build -t $(REGISTRY)/hing:$(GITCOMMIT) .
	@echo "==> Publishing $(REGISTRY)/hing:$(GITCOMMIT)"
	@docker push $(REGISTRY)/hing:$(GITCOMMIT)
	@echo "==> Your image is now available at $(REGISTRY)/hing:$(GITCOMMIT)"

clean:
	@echo "==> Cleaning releases"
	@rm hing

.PHONY: all release clean

