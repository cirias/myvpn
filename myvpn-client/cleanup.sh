#!/bin/bash

ip rule del fwmark 0x10 lookup 101

iptables -t mangle -D OUTPUT -p tcp --dport 6000 -j MARK --set-mark 0x10
iptables -t nat -D POSTROUTING -o tun0 -j SNAT --to-source 10.0.200.2
