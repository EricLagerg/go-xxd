#!/usr/bin/env bash
set -euf -o pipefail

DIRECTORY=$HOME/tmp
BIN=$HOME/bin

function cleanup {
  echo "Cleaning up..."
  rm --preserve-root -rf "${DIRECTORY}/go-xxd"
}
trap cleanup EXIT

if [[ -d "${DIRECTORY}" && ! -L "${DIRECTORY}" ]]; then

	echo "Entering ${DIRECTORY}"
	cd "${DIRECTORY}"
    git clone git@github.com:EricLagerg/go-xxd.git
    echo "cd go-xxd"
    cd go-xxd
    echo "building go-xxd"
    go build xxd.go

    if [[ -f  "${BIN}"/xxd ]]; then
    	echo "moving old xxd to xxd.bak"
    	mv "${BIN}/xxd" xxd.bak
	fi

    mv xxd "${BIN}"
fi

echo "xxd is: "$(which xxd)