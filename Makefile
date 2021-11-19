GOFLAGS=-trimpath -ldflags "-s -w -X 'github.com/harrybrwn/dots/cli.completions=false'"
NAME=$(shell basename $$PWD)
SOURCE=$(shell find . -name '*.go')
COMP=release/completion
BIN=release/bin/$(NAME)

build: $(BIN) gen

clean:
	$(RM) -r release

gen completion man:
	go run ./gen

ZSH_COMP=~/.config/zsh/oh-my-zsh/completions
BASH_COMP=~/.local/share/bash-completion/completions

install: $(BIN) gen
	@if [ ! -d $(BASH_COMP) ]; then mkdir -p $(BASH_COMP); fi
	@if [ ! -d $(ZSH_COMP) ];  then mkdir -p $(ZSH_COMP);  fi
	go install $(GOFLAGS)
	cp $(COMP)/bash/$(NAME) $(BASH_COMP)
	cp $(COMP)/zsh/_$(NAME) $(ZSH_COMP)

uninstall:
	go clean -i
	$(RM) $(ZSH_COMP)/_$(NAME)
	$(RM) $(BASH_COMP)/$(NAME)

.PHONY: build clean gen completion man install uninstall

image:
	docker image build -t dots:latest -f ./Dockerfile .

docker:
	docker container run -v $(shell pwd):/dots --rm -it dots sh

$(BIN): $(SOURCE)
	CGO_ENABLED=0 go build $(GOFLAGS) -o $@

$(COMP)/zsh/_$(NAME): $(COMP)/zsh $(BIN)
	$(BIN) completion zsh > $@

$(COMP)/bash/$(NAME): $(COMP)/bash $(BIN)
	$(BIN) completion bash > $@

$(COMP)/fish/$(NAME).fish: $(COMP)/fish $(BIN)
	$(BIN) completion fish > $@

$(COMP)/bash $(COMP)/zsh $(COMP)/fish:
	mkdir -p $@
