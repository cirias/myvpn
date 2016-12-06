## Overview
Simple VPN for linux.

## Install
Download the binary on the [release page](https://github.com/cirias/myvpn/releases).

## Usage

### Server

```bash
# start
sudo myvpn-server -secret=<yoursecret> -logtostderr -v=3

# setup NAT if you want to use the server's network
# replace `eth0` to your external interface
sudo iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
```

### Client

```bash
sudo myvpn-client -server-addr=<serverip>:9525 -secret=<yoursecret> -logtostderr -v=3
```

#### Use as default route

```bash
sudo ip route del default # you should backup the origin default route first

# replace tun0 to your local interface name
sudo ip route add default dev tun0
```

#### Use as systemd service

Create unit file `/usr/lib/systemd/system/myvpn-client.service`.

```
[Unit]
Description=Myvpn Client
Documentation=https://github.com/cirias/myvpn
After=network.target

[Service]
Type=simple
ExecStart=/opt/myvpn-client/myvpn-client -server-addr=<serverip>:9525 -password=<password> -logtostderr -v=2

[Install]
WantedBy=multi-user.target
```
