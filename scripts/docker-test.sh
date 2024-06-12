#!/bin/sh

set -eu

run() {
	echo "Running test $1"
	docker container run --rm \
		-v $(pwd):/dots:ro  \
		-e SSH_AUTH_SOCK=/ssh-auth-sock \
		-v "$SSH_AUTH_SOCK:/ssh-auth-sock" \
		--rm -it dots:test-latest "/dots/$1"
}

if [ -n "${1:-}" ]; then
	run "tests/test_$1.sh"
else
	for t in tests/test_*.sh; do
		echo "found test \"$t\""
	done
	for t in tests/test_*.sh; do
		run "$t"
	done
fi
