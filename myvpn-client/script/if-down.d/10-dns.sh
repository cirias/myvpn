#!/bin/bash

IF="$1"
LocalIP="$2"
ServerIP="$3"

resolvconf -d "$IF.*"
