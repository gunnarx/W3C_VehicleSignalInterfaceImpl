#!/bin/bash -e

MYDIR=$(dirname "$0")
#cd "$MYDIR"
GIT_ROOT=$(readlink -f "$MYDIR/../../../..")
echo "Project root is $GIT_ROOT"
cd "$GIT_ROOT/server/Go/src/server-1.0/server-core"

echo "Compiling server.  (If this was done already, there will be no output)"
go build -v .
