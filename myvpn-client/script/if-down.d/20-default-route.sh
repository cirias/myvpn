#!/bin/bash

IF="$1"
LocalIP="$2"
ServerIP="$3"

echo "ip route del $ServerIP"
ip route del $ServerIP
