FROM golang:1.17-alpine as builder

RUN mkdir /dots
WORKDIR /dots
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /usr/bin/dots

FROM alpine:3.14.3
COPY --from=builder /usr/bin/dots /usr/bin/dots
RUN apk --update add git
# RUN useradd --create-home -g users -G sudo dots
# USER dots
# ENTRYPOINT ["dots"]

