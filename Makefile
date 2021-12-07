NAME=$(shell basename $$PWD)
SOURCE=$(shell ./scripts/sourcehash.sh -e ./gen/main.go -l)
COMP=release/completion
BIN=release/bin/$(NAME)
RAW_VERSION=$(shell git describe --tags --abbrev=0)
VERSION=$(subst v,,$(RAW_VERSION))
COMMIT=$(shell git rev-parse HEAD)
HASH=$(shell ./scripts/sourcehash.sh -e ./gen/main.go)
GOFLAGS=-trimpath \
	-mod=mod      \
	-ldflags "-s -w \
		-X 'github.com/harrybrwn/dots/cli.completions=false'  \
		-X 'github.com/harrybrwn/dots/cli.Version=v$(VERSION)' \
		-X 'github.com/harrybrwn/dots/cli.Commit=$(COMMIT)'   \
		-X 'github.com/harrybrwn/dots/cli.Hash=$(HASH)'"
ARCH=$(shell go env GOARCH)

build: $(BIN) gen

clean:
	$(RM) -r release

gen completion man:
	go run ./gen

ZSH_COMP=~/.config/zsh/oh-my-zsh/completions
BASH_COMP=~/.local/share/bash-completion/completions
MAN_DIR_LOCAL=~/.local/share/man/man1

install: $(BIN) gen
	@if [ ! -d $(BASH_COMP) ]; then mkdir -p $(BASH_COMP); fi
	@if [ ! -d $(ZSH_COMP) ];  then mkdir -p $(ZSH_COMP);  fi
	@if [ ! -d $(MAN_DIR_LOCAL) ]; then mkdir -p $(MAN_DIR_LOCAL); fi
	install $(BIN) $$GOPATH/bin/
	cp $(COMP)/bash/$(NAME) $(BASH_COMP)
	cp $(COMP)/zsh/_$(NAME) $(ZSH_COMP)
	cp release/man/dots* $(MAN_DIR_LOCAL)

uninstall:
	go clean -i
	$(RM) $(ZSH_COMP)/_$(NAME)
	$(RM) $(BASH_COMP)/$(NAME)
	$(RM) ~/.local/share/man/man1/dots*

PKG=release/$(NAME)-$(VERSION)-$(ARCH)

package: $(PKG).deb

.PHONY: build clean gen completion man install uninstall package

image:
	docker image build -t dots:latest -f ./Dockerfile .

docker:
	docker container run -v $(shell pwd):/dots --rm -it dots bash

docker-test:
	@scripts/docker-test.sh

$(PKG).deb: $(PKG)/usr/bin/$(NAME)
	go run $(GOFLAGS) ./gen \
		-deb            \
		-package=$(PKG) \
		-name=$(NAME)   \
		-description='Manage your dotsfiles.'
	dpkg-deb --build $(PKG)

%/bin/$(NAME): $(SOURCE)
	CGO_ENABLED=0 go build -tags no_cobra_completion $(GOFLAGS) -o $@
