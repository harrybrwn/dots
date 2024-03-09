ARG UBUNTU_VERSION=20.04

FROM golang:1.22-alpine as builder

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
        openssh         \
        less
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
ENV PAGER=less
ENV LESS=--raw-control-chars
VOLUME /dots
WORKDIR /dots

FROM ubuntu:${UBUNTU_VERSION} as test
RUN yes | unminimize && \
    apt update && \
    apt install -yq git vim && \
    groupadd dots && \
    useradd  \
      --create-home         \
      --shell /usr/bin/bash \
      --home-dir /home/dots \
      --gid dots            \
      --groups dots,root    \
      dots && \
    echo "alias l='ls -la --group-directories-first' \
                la='ls -A --group-directories-first'" >> /home/dots/.bashrc && \
    mkdir /home/dots/.ssh/ && \
    ssh-keyscan -t rsa github.com >> /home/dots/.ssh/known_hosts && \
    chown -R dots:dots /home/dots/.ssh
ARG DEFAULT_REPO=git@github.com:harrybrwn/dotfiles.git
ENV DOTS_DEFAULT_REPO=${DEFAULT_REPO}
RUN echo "alias dots-i='dots install -y ${DEFAULT_REPO}'" >> /home/dots/.bashrc
COPY --from=builder /dots/release/bin/dots /usr/bin/dots
USER dots
WORKDIR /home/dots
ENV PAGER=less \
    EDITOR=vim \
    LESS=--raw-control-chars \
    XDG_CONFIG_HOME="/home/dots/.config"

