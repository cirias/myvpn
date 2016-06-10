#!/bin/bash

IF="$1"
ServerIP="$2"

echo "nameserver 8.8.8.8" | resolvconf -a "$IF.inet"
