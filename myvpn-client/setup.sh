#!/bin/bash

ip route add default dev tun0 mtu 1280 table 101

ip rule add fwmark 0x10 lookup 101

iptables -t mangle -A OUTPUT -p tcp --dport 6000 -j MARK --set-mark 0x10
iptables -t nat -A POSTROUTING -o tun0 -j SNAT --to-source 10.0.200.2

sysctl -w net.ipv4.conf.tun0.rp_filter=2
