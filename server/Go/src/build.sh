#!/bin/bash -e

MYDIR="$(dirname "$0")"
cd "$MYDIR"
GIT_ROOT=$(readlink -f "$PWD/../../..")
echo "Project root is $GIT_ROOT"
cd "$GIT_ROOT/server/Go/src/server-core/src"

# This is required to find several imports
export GOPATH="$GOPATH:$GIT_ROOT/server/Go/"

# Downloaded dependencies seem to end up in $GIT_ROOT/src
# which means GOPATH must reference $GIT_ROOT?
export GOPATH="$GOPATH:$GIT_ROOT"

echo "GOPATH: $GOPATH"

echo "Compiling server.  (If this was done already, there will be no output)"
go build -v .
