#!/bin/bash

set -e

mkdir /dev/net && \
  mknod /dev/net/tun c 10 200

exec "$@"
