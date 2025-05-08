#!/usr/bin/env bash

set -euo pipefail

source "$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)/common.sh"

# export PAGER='/usr/bin/less'

#dots clone git@github.com:thoughtbot/dotfiles.git
# dots clone https://github.com/thoughtbot/dotfiles.git
#dots clone git@github.com:pwyde/dotfiles.git

# dots clone https://github.com/hlissner/dotfiles.git
dots clone git@github.com:hlissner/dotfiles.git
dots install -y

for f in shell.nix README.md flake.nix flake.lock LICENSE; do
	test -f ~/$f
done
for d in templates packages overlays config bin lib hosts; do
	if [ ! -d ~/$d ]; then
		echo "dir $d does not exist"
		exit 1
	fi
done
