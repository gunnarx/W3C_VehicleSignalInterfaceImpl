#!/bin/bash -e

MYDIR=$(dirname "$0")
#cd "$MYDIR"
GIT_ROOT=$(readlink -f "$MYDIR/../../..")
echo "Project root is $GIT_ROOT"
cd "$GIT_ROOT/server/Go/src/server-core/src"

# This is required to find several imports
export GOPATH="$GOPATH:$GIT_ROOT/server/Go/"
# This is required to find proto_files import
export GOPATH="$GOPATH:$GIT_ROOT/server/Go/src/server-core"

echo "GOPATH: $GOPATH"

echo "Compiling server.  (If this was done already, there will be no output)"
go build -v .
