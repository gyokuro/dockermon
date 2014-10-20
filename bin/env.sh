#!/bin/bash

PROJECT="github.com/gyokuro/dockermon"
echo "Current directory is $(pwd)"

if [ ! -d "$HOME/go/src/$PROJECT" ]; then
    echo "Creating $HOME/go as the root of go development and set up symlinks to point to this directory."
    IFS='/' read -a proj <<< "$PROJECT"
    mkdir -p $HOME/go/src/${proj[0]}/${proj[1]}
    ln -s $(pwd) $HOME/go/src/${proj[0]}/${proj[1]}/${proj[2]}
fi

export GOPATH=$HOME/go
export PATH=$HOME/go/bin:$PATH

# Godep dependency manager
if [[ $(which godep) == "" ]]; then
    echo "Installing godep."
    go get github.com/tools/godep
fi

# Development tools
if [[ "$CIRCLE_ENV" != "" ]]; then
    echo "Setting up development environment."
    # Run go oracle for development https://godoc.org/code.google.com/p/go.tools/oracle
    if [[ $(which oracle) == "" ]]; then
	echo "Setting up go oracle for source code analysis."
	go install code.google.com/p/go.tools/cmd/oracle
    fi

    if [[ $(which godoc) == "" ]]; then
	echo "Godoc not installed."
    fi
fi
