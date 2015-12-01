#!/bin/bash

rm myvpn-client.tgz

go build && \
  tar -czvf myvpn-client.tgz myvpn-client script ../script && \
  rm myvpn-client
