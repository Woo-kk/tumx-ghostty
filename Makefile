VERSION ?= dev
DIST_DIR ?= $(CURDIR)/dist/release/$(VERSION)

.PHONY: test build release-binaries package install-tarball homebrew-formula publish-homebrew-tap clean

test:
	go test ./...

build:
	go build ./cmd/tmux-ghostty
	go build ./cmd/tmux-ghostty-broker

release-binaries:
	./scripts/build-release.sh $(VERSION)

package: release-binaries homebrew-formula
	./scripts/build-pkg.sh $(VERSION)

install-tarball: release-binaries
	./scripts/install-tarball.sh --archive "$(DIST_DIR)/tmux-ghostty_$(VERSION)_darwin_universal.tar.gz"

homebrew-formula: release-binaries
	./scripts/build-homebrew-formula.sh $(VERSION)

publish-homebrew-tap: homebrew-formula
	./scripts/publish-homebrew-tap.sh $(VERSION)

clean:
	rm -rf dist
