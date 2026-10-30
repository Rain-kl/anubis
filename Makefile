VERSION= $(shell cat ./VERSION)
GO?= go
NPM?= npm

.PHONY: build assets deps lint prebaked-build test canary

all: build

deps:
	$(NPM) ci
	$(GO) mod download

assets: PATH:=$(PWD)/node_modules/.bin:$(PATH)
assets: deps
	$(GO) generate ./...
	bash ./wasm/scripts/build_wasm.sh
	bash ./wasm/scripts/build_wasm2js.sh
	./web/build.sh
	./xess/build.sh

build: assets
	$(GO) build -o ./var/anubis ./cmd/anubis
	$(GO) build -o ./var/robots2policy ./cmd/robots2policy
	@echo "Anubis is now built to ./var/anubis"

lint: assets
	$(GO) vet ./...
	$(GO) tool staticcheck ./...
	
prebaked-build:
	$(GO) build -buildvcs=false -o ./var/anubis -ldflags "-X 'github.com/TecharoHQ/anubis.Version=$(VERSION)'" ./cmd/anubis
	$(GO) build -buildvcs=false -o ./var/robots2policy -ldflags "-X 'github.com/TecharoHQ/anubis.Version=$(VERSION)'" ./cmd/robots2policy

test: assets
	$(GO) test ./...

# Merge the current local branch into canary, push canary, then restore it.
# Dirty worktrees are auto-stashed and restored after the operation.
canary:
	@set -e; \
	if ! git rev-parse --git-dir >/dev/null 2>&1; then \
		echo "Error: not a git repository"; exit 1; \
	fi; \
	orig=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$orig" = "HEAD" ]; then \
		echo "Error: detached HEAD; checkout a branch first"; exit 1; \
	fi; \
	if [ "$$orig" = "canary" ]; then \
		echo "Error: already on canary; checkout a source branch first"; exit 1; \
	fi; \
	stashed=0; \
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "Working tree is dirty; stashing local changes..."; \
		git stash push -u -m "make canary auto-stash from $$orig"; \
		stashed=1; \
	fi; \
	cleanup() { \
		cur=$$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true); \
		if [ "$$cur" != "$$orig" ]; then \
			echo "Restoring branch $$orig..."; \
			git checkout -q "$$orig"; \
		fi; \
		if [ "$$stashed" = "1" ]; then \
			echo "Restoring stashed local changes..."; \
			git stash pop; \
		fi; \
	}; \
	trap cleanup EXIT; \
	echo "Merging $$orig -> canary..."; \
	if git show-ref --verify --quiet refs/heads/canary; then \
		git checkout canary; \
	else \
		echo "canary branch does not exist yet; creating it from $$orig..."; \
		git checkout -b canary; \
	fi; \
	git merge --no-edit "$$orig"; \
	git push -u origin canary; \
	echo "Done: $$orig merged into canary and pushed."
