#!/bin/bash

IF="$1"
LocalIP="$2"
ServerIP="$3"

ip route add $(ip route get $ServerIP | head -n1)
