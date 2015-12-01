## TODO
- [x] Rename project
- [x] Clean up code
  - [x] server.go
  - [x] error handle
  - [x] replace `log` to `glog`
  - [ ] refactor log format
- [x] Create hook script
  - [x] create `conf.d` for both server and client
  - [x] ip route for server address
  - [x] set mtu
  - [x] Excute script under dir
  - [x] excute if-down when system signal reach
- [ ] Choose default port
- [ ] Write documents
- [x] Change to use UDP
- [x] Add expire for client
- [x] Replace ippool and portpool with channel
- [ ] Robust
  - [ ] Integrity (hmac)
- [ ] Refactor UDP port logic

## Usage

### Server
```bash
# start
sudo myvpn-server -password=<yourpassword> -logtostderr -v=2

# setup NAT if you want to use the server's network
# replace `eth0` to your external interface
sudo iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
```

### Client
```bash
sudo myvpn-client -server-addr=<serverip>:9525 -password=<yourpassword> -logtostderr -v=2

# set as default route
# replace 10.0.200.1 to your server internal ip
# replace tun0 to your local interface name
sudo ip route add default via 10.0.200.1 dev tun0
```
