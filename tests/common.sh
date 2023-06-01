inside_docker() {
    if ! grep -i docker /proc/1/cgroup > /dev/null; then
        return 1
    fi
    if [ ! -f /.dockerenv ]; then
        return 1
    fi
    return 0
}

if ! inside_docker; then
	echo 'Error: not running inside docker container'
	exit 1
fi
