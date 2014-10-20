#!/bin/bash

# Git commit hash / message
GIT_COMMIT_HASH=$(git rev-list --max-count=1 --reverse HEAD)
GIT_COMMIT_MESSAGE=$(git log -1 | tail -1 | sed -e "s/^[ ]*//g")
BUILD_TIMESTAMP=$(date +"%Y-%m-%d-%H:%M")

echo "Git commit $GIT_COMMIT_HASH ($GIT_COMMIT_MESSAGE) on $BUILD_TIMESTAMP"
sed -ri "s/@@GIT_COMMIT_HASH@@/${GIT_COMMIT_HASH}/g" pkg/build_info.go
sed -ri "s/@@GIT_COMMIT_MESSAGE@@/${GIT_COMMIT_MESSAGE}/g" pkg/build_info.go
sed -ri "s/@@BUILD_TIMESTAMP@@/${BUILD_TIMESTAMP}/g" pkg/build_info.go

cat pkg/build_info.go

WORKING=$(pwd)

# Usage: ./build_xcompile.sh src/*.go

# Script for cross-compiling go binaries for different platforms
PLATFORMS="darwin/386 linux/amd64 windows/amd64"

eval "$(go env)"

# Input source code .go files to build
SRC=$@

echo "Cleaning target"
rm -rf target

for PLATFORM in $PLATFORMS; do
	export GOOS=${PLATFORM%/*}
	export GOARCH=${PLATFORM#*/}	
	TARGET=target/${GOOS}_${GOARCH}
	echo "Building for ${GOOS} on ${GOARCH}: ${TARGET}"
	mkdir -p ${TARGET}
	pushd ${TARGET}
	for s in $SRC; do
		go build $GOPATH/$s
	done
	popd
done

# Make sure the GOOS/GOARCH environment variables are set correctly
eval "$(go env)"

echo "Finished. Built artifacts are in 'target':"
find ./target
for i in $(find ./target); do 
    if [ -f $i ]; then
	echo "Generated binary - $(file $i)"
    fi; 
done


