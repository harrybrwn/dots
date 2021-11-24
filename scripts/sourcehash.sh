#!/bin/sh

set -e

DEBUG=false
LIST=false
EXCLUDE=''

usage() {
  echo 'sourcehash.sh [[-l|--list]|-e <pattern>]'
}

while :; do
  case $1 in
    -h|--help)
      usage
      exit
      ;;
    -l|--list)
      LIST=true
      shift
      ;;
    -e|--exclude)
      if [ -z "$EXCLUDE" ];then
        EXCLUDE="$2"
      else
        EXCLUDE="$EXCLUDE $2"
      fi
      shift 2
      ;;
    -d)
      DEBUG=true
      shift
      ;;
    *)
      break
      ;;
  esac
done

list() {
  local e=""
  if [ -n "$EXCLUDE" ]; then
    for f in $EXCLUDE; do
      # echo "exclude: $f"
      e="-not -path $f $e"
    done
  fi

  find .                        \
    \(                          \
      -name '*.go'              \
      -o -name 'go.mod'         \
      -o -name 'go.sum'         \
    \)                          \
    -type f                     \
    -not -path './test/*' $e
}

if $DEBUG; then
  usage
  echo
  list
  exit
fi

if $LIST; then
  list | sort -d -s
  exit
fi


list         |
  sort -d -s |
  xargs cat  |
  sha256sum  |
  sed -Ee 's/\s|-//g'
