#!/bin/bash

IF="$1"
LocalIP="$2"
ServerIP="$3"

echo "nameserver 8.8.8.8" | resolvconf -a "$IF.inet"
