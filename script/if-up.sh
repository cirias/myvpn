#!/bin/bash

# This script is called with the following arguments
# Arg Name

IF="$1"
LocalIP="$2"
ServerIP="$3"

# SETUP FUNCTIONS
ip_addr() {
  ip addr add "$2" dev "$1"
}

ip_link() {
  # 1432 = 1500 - 60 - 8 - 16
  #ip link set dev "$1" mtu 1416
  ip link set dev "$1" up
}

# MAIN()

ip_addr "$IF" "$LocalIP"

ip_link "$IF"
