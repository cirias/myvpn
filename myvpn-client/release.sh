#!/bin/bash

rm myvpn-client.tgz
mkdir -p .tmp/myvpn-client/

go build
mv myvpn-client .tmp/myvpn-client/
cp -r ./script/ .tmp/myvpn-client/
cp -r ../script/ .tmp/myvpn-client/
#chown -R root:root .tmp/myvpn-client/
cd .tmp/ && tar -czvf myvpn-client.tgz myvpn-client/ && cd ..
mv .tmp/myvpn-client.tgz ./
rm -r .tmp/
