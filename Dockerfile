FROM golang:1.17-alpine as builder

RUN apk update && \
    apk add git shadow bash make vim openssh

RUN mkdir /app
WORKDIR /app
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download

RUN mkdir ~/.vim ~/.ssh && \
    # SSH
    #ssh-keyscan github.com >> ~/.ssh/known_hosts && \
    git config --global url."git@github.com:".insteadof "https://github.com/" && \
    # Vim
    echo "set number" >> ~/.vim/vimrc && \
    # Bash
    echo "PS1='\u@\h:\w \$ '" >> ~/.bashrc && \
    echo "alias l='ls -la --group-directories-first' \
                la='ls -A --group-directories-first'" >> ~/.bashrc

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /usr/bin/dots

VOLUME /dots
WORKDIR /dots
