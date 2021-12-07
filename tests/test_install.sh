#!bin/bash

set -e

source "$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)/common.sh"

if ! inside_docker; then
	echo 'Error: not running inside docker container'
	exit 1
fi

# export PAGER='/usr/bin/less'

#dots clone git@github.com:thoughtbot/dotfiles.git
#dots clone https://github.com/thoughtbot/dotfiles.git
dots clone https://github.com/hlissner/dotfiles.git
dots install -y
dots update
dots util modified