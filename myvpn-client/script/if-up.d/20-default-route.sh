#!/bin/bash

IF="$1"
ServerIP="$2"

ip route add $(ip route get $ServerIP | head -n1)
