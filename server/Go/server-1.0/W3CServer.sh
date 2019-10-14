#!/bin/bash

if [ "$0" != bash ] ; then
  echo "This script should be *sourced* in your interactive shell (it is assuming bash)"
  echo "Please run:  . $0 or source $0"
  exit 1
fi

# Helper function
add_gopath() {
  local newpath="$1"
  if [[ ! "$GOPATH" =~ (^|:)$newpath/?(:|$) ]]; then
    echo "Adding $newpath to \$GOPATH"
    if [ -z "$GOPATH" ] ; then
      export GOPATH="$newpath"         # Was empty
    else
      export GOPATH="$GOPATH:$newpath" # Not empty, add to end
    fi
    echo "GOPATH is now : $GOPATH"
  fi
}

# Logic to make this location independent, i.e. can be run
# from any directory.

# This thing gives us the path to this script, even if sourced...
S=$(echo "${BASH_SOURCE[${#BASH_SOURCE[@]} - 1]}")
D=$(dirname "$S")  # Capture the (relative) dir...
# ... and get an absolute path out of that
export W3C_SERVER_DIR="$(readlink -f "$D")"
GIT_ROOT=$(readlink -f "$W3C_SERVER_DIR/../../..")
echo "Project root is $GIT_ROOT"

# The way http_mgr is written, it includes server-1.0/utils,
# which must be then found relative to some part of $GOPATH
# so therefore:
add_gopath "$GIT_ROOT/server/Go"

echo Setting up startme and stopme convenience functions

startme() {
    screen -d -m -S serverCore bash -c "cd $W3C_SERVER_DIR/server-core && go build && ./server-core"
    screen -d -m -S serviceMgr bash -c "cd $W3C_SERVER_DIR && go run service_mgr.go"
    screen -d -m -S wsMgr bash -c "cd $W3C_SERVER_DIR && go run ws_mgr.go"
    screen -d -m -S httpMgr bash -c "cd $W3C_SERVER_DIR && go run http_mgr.go"
}

stopme() {
    screen -X -S httpMgr quit
    screen -X -S wsMgr quit
    screen -X -S serviceMgr quit
    screen -X -S serverCore quit
    #screen -wipe
}

configureme() {
    #ln -s <absolute-path-to-dir-of-git-root>/W3C_VehicleSignalInterfaceImpl/server/Go/server-1.0 $GOPATH/src/server-1.0
}

if [ $1 = startme ]
then
startme
fi

if [ $1 = stopme ]
then
stopme
fi

if [ $1 = configureme ]
then
configureme
fi

