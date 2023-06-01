#!/usr/bin/env bash

set -euo pipefail

source "$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)/common.sh"

dots clone git@github.com:hlissner/dotfiles.git
dots install -y
for f in shell.nix README.md flake.nix flake.lock LICENSE; do
	test -f ~/$f
done
for d in templates packages overlays config bin lib hosts; do
	test -d ~/$d
done

dots uninstall

for f in shell.nix README.md flake.nix flake.lock LICENSE; do
	test ! -f ~/$f
done
for d in templates packages overlays config bin lib hosts; do
	test ! -d ~/$d
done