#!/bin/bash

# This script is called with the following arguments
# Arg Name
# $1 Interface name
# $2 MTU
# $3 Local IP number

DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

for f in `ls $DIR/if-down.d/*.sh`; do
  echo "$f" "$@"
  bash "$f" "$@"
done
