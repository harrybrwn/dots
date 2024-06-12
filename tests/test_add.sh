#!/bin/bash

set -e

source "$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)/common.sh"

if ! inside_docker; then
	echo 'Error: not running inside docker container'
	exit 1
fi

git="git --git-dir /root/.dots/repo --work-tree /root"

export PAGER=less LESS='--raw-control-chars'

git --no-pager config --global 'commit.gpgsign' 'false'

#dots clone /dots/test/config/repo
#dots git remote remove origin
#echo '# .bashrc' > ~/.bashrc
#dots install -y
#dots update

echo '# ~/.bashrc' > ~/.bashrc
dots add ~/.bashrc

commits="$($git --no-pager log --no-color --oneline --all | wc -l)"
if [[ $commits != 1 ]]; then
    git --no-pager log --no-color --oneline --all
    echo 'expected one commit message'
    exit 1
fi

cat > ~/testfile.txt <<-EOF
this is a test file
here is some more test file stuff
EOF

dots add ~/testfile.txt

cat >> ~/testfile.txt <<-EOF
More stuff appended
EOF

if [[ "$(dots util modified | wc -l)" != 2 ]]; then
    echo 'Did not find modified file' 1>&2
    exit 1
fi

dots update

if [[ "$(dots util modified | wc -l)" != 0 ]]; then
    echo 'Should not find modified files' 1>&2
    exit 1
fi