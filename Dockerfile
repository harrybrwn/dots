FROM golang:1.17-alpine as builder

RUN apk update && \
    apk add git shadow bash make vim

RUN mkdir /app
WORKDIR /app
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download

RUN mkdir ~/.vim && \
    echo "set number" >> ~/.vim/vimrc && \
    echo "PS1='\u@\h:\w \$ '" >> ~/.bashrc && \
    echo "alias l='ls -la --group-directories-first' \
                la='ls -A --group-directories-first'" >> ~/.bashrc

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /usr/bin/dots

VOLUME /dots
WORKDIR /dots
