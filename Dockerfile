FROM golang:1.17-alpine as builder

RUN apk update && apk add git make
WORKDIR /dots
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download
COPY . .
RUN make build

FROM alpine:3.17 as dots
RUN apk update && \
    apk add             \
        git             \
        shadow          \
        bash            \
        bash-completion \
        make            \
        vim             \
        openssh
RUN mkdir ~/.vim ~/.ssh && \
    # SSH
    ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts && \
    git config --global url."git@github.com:".insteadof "https://github.com/" && \
    # Vim
    echo "set number" >> ~/.vim/vimrc && \
    # Bash
    echo "PS1='\u@\h:\w \$ '" >> ~/.bashrc && \
    echo "alias l='ls -la --group-directories-first' \
                la='ls -A --group-directories-first'" >> ~/.bashrc
COPY --from=builder /dots/release/bin/dots /usr/bin/dots
COPY --from=builder /dots/release/completion/bash/dots /usr/share/bash-completion/completions

VOLUME /dots
WORKDIR /dots
