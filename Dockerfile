FROM golang:1.17-alpine as builder

RUN apk --update add git

RUN mkdir /app
WORKDIR /app
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /usr/bin/dots
WORKDIR /
