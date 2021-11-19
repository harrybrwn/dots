GOFLAGS=-trimpath -ldflags "-s -w"
NAME=$(shell basename $$PWD)
SOURCE=$(shell find . -name '*.go')
COMP=release/completion
BIN=release/bin/$(NAME)

build: $(BIN) gen

clean:
	$(RM) -r release

# completion: $(COMP)/bash/$(NAME) $(COMP)/zsh/_$(NAME) $(COMP)/fish/$(NAME).fish
gen completion man:
	go run ./gen

install:
	go install $(GOFLAGS)
	sudo cp $(COMP)/zsh/_$(NAME) /usr/share/zsh/vendor-completions/
	sudo cp $(COMP)/bash/$(NAME) /usr/share/bash-completion/completions

uninstall:
	go clean -i
	sudo $(RM) /usr/share/bash-completion/completions/$(NAME)
	sudo $(RM) /usr/share/zsh/vendor-completions/_$(NAME)

.PHONY: build clean gen completion man install uninstall

$(BIN): $(SOURCE)
	go build $(GOFLAGS) -o $@

$(COMP)/zsh/_$(NAME): $(COMP)/zsh $(BIN)
	$(BIN) completion zsh > $@

$(COMP)/bash/$(NAME): $(COMP)/bash $(BIN)
	$(BIN) completion bash > $@

$(COMP)/fish/$(NAME).fish: $(COMP)/fish $(BIN)
	$(BIN) completion fish > $@

$(COMP)/bash $(COMP)/zsh $(COMP)/fish:
	mkdir -p $@


