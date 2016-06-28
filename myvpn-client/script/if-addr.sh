#!/bin/bash

# This script is called with the following arguments
# Arg Name

IF="$1"
LocalIP="$2"

echo "bringup $IF $LocalIP"

# SETUP FUNCTIONS
ip_addr() {
  ip addr flush dev "$1"
  ip addr add "$2" dev "$1"
}

# MAIN()

ip_addr "$IF" "$LocalIP"
