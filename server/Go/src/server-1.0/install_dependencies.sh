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
go get -v ./...
set -e
# The above has some errors, return success anyway for now
exit 0
