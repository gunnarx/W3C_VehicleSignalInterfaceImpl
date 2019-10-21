#!/bin/bash -e
MYDIR=$(dirname "$0")
#cd "$MYDIR"
GIT_ROOT=$(readlink -f "$MYDIR/../../../..")
echo "Project root is $GIT_ROOT"
cd "$GIT_ROOT"

echo Getting packages
echo
echo "(NOTE - This will report errors for the local imports)"
set +e

# This is required to find several imports
export GO_PATH="$GO_PATH:$GIT_ROOT/server/Go/src/server-1.0/"
# This is required to find proto_files import
export GO_PATH="$GO_PATH:$GIT_ROOT/server/Go/src/server-1.0/server-core"

# Fetch all dependencies
go get -v ./...

set -e
# The above has some errors, return success anyway for now
exit 0
