NAME=$(shell basename $$PWD)
SOURCE=$(shell ./scripts/sourcehash.sh -e ./gen/main.go -l)
COMP=release/completion
BIN=release/bin/$(NAME)
RAW_VERSION=$(shell git describe --tags --abbrev=0)
DATE=$(shell date +%FT%T%:z)
VERSION=$(subst v,,$(RAW_VERSION))
COMMIT=$(shell git rev-parse HEAD)
HASH=$(shell ./scripts/sourcehash.sh -e './cmd/gen/*')
GOFLAGS=-trimpath \
	-mod=mod      \
	-ldflags "-s -w \
		-X 'github.com/harrybrwn/dots/cli.completions=false'  \
		-X 'github.com/harrybrwn/dots/cli.Version=v$(VERSION)' \
		-X 'github.com/harrybrwn/dots/cli.Commit=$(COMMIT)'   \
		-X 'github.com/harrybrwn/dots/cli.Hash=$(HASH)' \
		-X 'github.com/harrybrwn/dots/cli.Date=$(DATE)'"
ARCH=$(shell go env GOARCH)
DIST=release

build: $(BIN) gen

clean:
	$(RM) -r release dist result

gen completion man:
	go run ./cmd/gen -name=$(NAME)

ZSH_COMP=~/.config/zsh/oh-my-zsh/completions
BASH_COMP=~/.local/share/bash-completion/completions
MAN_DIR_LOCAL=~/.local/share/man/man1

install: $(BIN) gen
	@if [ ! -d $(BASH_COMP) ]; then mkdir -p $(BASH_COMP); fi
	@if [ ! -d $(ZSH_COMP) ];  then mkdir -p $(ZSH_COMP);  fi
	@if [ ! -d $(MAN_DIR_LOCAL) ]; then mkdir -p $(MAN_DIR_LOCAL); fi
	# install $(BIN) $$GOPATH/bin/
	cp $(BIN) $$GOPATH/bin/
	cp $(COMP)/bash/$(NAME) $(BASH_COMP)
	cp $(COMP)/zsh/_$(NAME) $(ZSH_COMP)
	cp release/man/dots* $(MAN_DIR_LOCAL)

uninstall:
	go clean -i
	$(RM) $(ZSH_COMP)/_$(NAME)
	$(RM) $(BASH_COMP)/$(NAME)
	$(RM) ~/.local/share/man/man1/dots*

.PHONY: snapshot
snapshot:
	goreleaser release --skip-publish --skip-announce --auto-snapshot --rm-dist

test:
	go test -cover ./...

PKG=release/$(NAME)-$(VERSION)-$(ARCH)

package: $(PKG).deb

.PHONY: build clean gen completion man install uninstall package

# This is not really a lock file, its just a file that is created whenever we
# build the docker image an is used as a makefile depenancy for the docker
# image.
IMAGE_LOCK=$(DIST)/.docker-build-lock

image: $(IMAGE_LOCK)

container: $(IMAGE_LOCK)
	docker container run                  \
		-e SSH_AUTH_SOCK=/ssh-auth-sock   \
		-v $$SSH_AUTH_SOCK:/ssh-auth-sock \
		-v $(shell pwd):/dots:ro          \
		-w /root                          \
		--rm -it dots bash

docker-test: $(IMAGE_LOCK)
	@scripts/docker-test.sh

DOCKER_TEST_SCRIPTS=$(shell find ./tests -name 'test_*.sh')
GOFILES=$(shell scripts/sourcehash.sh -e '*_test.go' -e './cmd/gen/*' -l)

$(IMAGE_LOCK): Dockerfile $(GOFILES) $(DOCKER_TEST_SCRIPTS)
	@if [ ! -d $(DIST) ]; then mkdir $(DIST); fi
	docker image build -t dots:latest -f ./Dockerfile .
	@touch $@

$(PKG).deb: $(PKG)/usr/bin/$(NAME)
	go run $(GOFLAGS) ./cmd/gen \
		-deb            \
		-package=$(PKG) \
		-name=$(NAME)   \
		-description='Manage your dotsfiles.'
	dpkg-deb --build $(PKG)

%/bin/$(NAME): $(SOURCE)
	CGO_ENABLED=0 go build $(GOFLAGS) -o $@ ./cmd/dots
