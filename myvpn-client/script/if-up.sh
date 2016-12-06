#!/bin/bash

# This script is called with the following arguments
# Arg Name

IF="$1"
ServerIP="$2"

# SETUP FUNCTIONS
ip_link() {
  # 1432 = 1500 - 60 - 8 - 16
  #ip link set dev "$1" mtu 1416
  ip link set dev "$1" up
}

# MAIN()

ip_link "$IF"

DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

for f in `ls $DIR/if-up.d/*.sh`; do
  echo "$f" "$@"
  bash "$f" "$@"
done
