#!/bin/bash -e
MYDIR=$(dirname "$0")
#cd "$MYDIR"
GIT_ROOT="$(readlink -f "$MYDIR/../../..")"
echo "Project root is $GIT_ROOT"
cd "$GIT_ROOT"

echo Getting packages
echo
echo "(NOTE - This will report errors for the local imports)"
set +e

echo "GOROOT: $GOROOT"
unset GOROOT

# This is required to find several imports
export GOPATH="$GOPATH:$GIT_ROOT/server/Go/"
# This is required to find proto_files import
export GOPATH="$GOPATH:$GIT_ROOT/server/Go/src/server-core"

export GOPATH="$GOPATH:$GIT_ROOT/server/Go/"

echo "GOPATH: $GOPATH"
# Fetch all dependencies
go get -v ./...
set -e

# The above has some errors, return success anyway for now
exit 0
