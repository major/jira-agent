GORELEASER ?= goreleaser
GORELEASER_SNAPSHOT_ARGS ?= --snapshot --clean --skip=sign
GOVULNCHECK ?= go run golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: build test smoke lint vuln clean install release-check release-snapshot release

build:
	go build -o jira-agent ./cmd/jira-agent/

test:
	go test -v -race -shuffle=on -coverprofile=coverage.out ./...

SMOKE_PROJECT ?= RSPEED
SMOKE_ISSUE ?= RSPEED-2229

smoke: build
	SMOKE_PROJECT=$(SMOKE_PROJECT) SMOKE_ISSUE=$(SMOKE_ISSUE) \
		go test -v -tags smoke_live -count=1 -timeout=120s ./cmd/jira-agent/

lint:
	golangci-lint run ./...

vuln:
	$(GOVULNCHECK) ./...

clean:
	go clean
	rm -f jira-agent coverage.out
	rm -rf dist/

install:
	go install ./cmd/jira-agent/

release-check:
	$(GORELEASER) check

release-snapshot: release-check
	$(GORELEASER) release $(GORELEASER_SNAPSHOT_ARGS)

release: release-check
ifndef VERSION
	$(error VERSION is required. Usage: make release VERSION=v0.1.0)
endif
	@[ "$$(git branch --show-current)" = "main" ] || { echo "Error: must be on main branch"; exit 1; }
	@[ -z "$$(git status --porcelain)" ] || { echo "Error: working tree is not clean"; exit 1; }
	@$(MAKE) test
	@$(MAKE) lint
	@$(MAKE) vuln
	@PREV=$$(git describe --tags --abbrev=0 2>/dev/null || true); \
	if [ -n "$$PREV" ]; then RANGE="$$PREV..HEAD"; else RANGE=""; fi; \
	FEATS=$$(git log --oneline --grep='^feat' $$RANGE | sed 's/^[a-f0-9]* /- /'); \
	FIXES=$$(git log --oneline --grep='^fix' $$RANGE | sed 's/^[a-f0-9]* /- /'); \
	OTHER=$$(git log --oneline --grep='^feat' --grep='^fix' --invert-grep $$RANGE | sed 's/^[a-f0-9]* /- /'); \
	MSG="Release $(VERSION)"; \
	if [ -n "$$FEATS" ]; then MSG="$$MSG\n\nFeatures:\n$$FEATS"; fi; \
	if [ -n "$$FIXES" ]; then MSG="$$MSG\n\nFixes:\n$$FIXES"; fi; \
	if [ -n "$$OTHER" ]; then MSG="$$MSG\n\nOther:\n$$OTHER"; fi; \
	printf '%b\n' "$$MSG" | git tag -s -F - $(VERSION)
	@echo ""
	@echo "Tag $(VERSION) created. Push with: git push origin $(VERSION)"
