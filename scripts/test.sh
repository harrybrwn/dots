#!/bin/bash

set -eu

build_args=(
  # --build-arg DEFAULT_REPO=git@github.com:harrybrwn/dotfiles.git
  # --build-arg DEFAULT_REPO=git@github.com:mathiasbynens/dotfiles.git
  --build-arg DEFAULT_REPO=git@github.com:WhoIsSethDaniel/dotfiles.git
)

docker image build \
  --progress plain \
  --build-arg UBUNTU_VERSION=20.04 \
  ${build_args[@]} \
  --target 'test' \
  -f ./Dockerfile \
  -t dots:latest-ubuntu \
  .

docker container run \
  -e SSH_AUTH_SOCK=/ssh-auth-sock \
  -v "$SSH_AUTH_SOCK:/ssh-auth-sock" \
  --rm -it dots:latest-ubuntu bash
