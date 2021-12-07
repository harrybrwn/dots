#!/bin/sh

set -e

for t in $(ls -A tests); do
	if [ "$t" != "common.sh" ]; then
		echo "running test $t"
		docker container run --rm -v $(pwd):/dots:ro --rm -it dots "/dots/tests/$t"
	fi
done
