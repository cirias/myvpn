## TODO
- [x] Rename project
- [x] Clean up code
  - [x] server.go
  - [x] error handle
  - [x] replace `log` to `glog`
- [x] Create hook script
  - [x] create `conf.d` for both server and client
  - [x] ip route for server address
  - [x] set mtu
  - [x] Excute script under dir
  - [x] excute if-down when system signal reach
- [x] Change to use UDP
- [x] Add expire for client
- [x] Replace ippool and portpool with channel
- [x] Use random key for each client
- [x] Rename constants and error code
- [x] Merge portpool with ippool
- [x] Replace client timer with client collection
- [x] Handle tun error `invalid argument`
- [x] Refactor log format
- [ ] Dockerize
- [ ] Choose default port
- [ ] Write documents
- [ ] Systemd service

## Install
Download the binary on the [release page](https://github.com/cirias/myvpn/releases).
Extract the archive.

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
# replace tun0 to your local interface name
sudo ip route add default dev tun0
```

## Tech
The VPN include two parts. First, client handshake with server.
After handshake success, communication start.

### Handshake
Handshake start by client creating `TCP` connection to server.
Then client generate a random key used as the key of `aes`.
Client send the random key to server. The random key should be
encrypt with pre-shared password.

```
+----------------------+
|    |  Encrypted Data |
| IV |-----------------|
|    | IV | random key |
|----+----+------------|
| 16 | 16 |     32     |
+----------------------+
```

If the password valid, server will send back the data.

```
+----------------------------+
|    |     Encrypted Data    |
| IV |-----------------------|
|    | IP | IPMask | UDPPort |
|----+----+--------+---------|
| 16 |  4 |    4   |    2    |
+----------------------------+
```

- **IP** is the internal IP of client allocated by server.
- **IPMask** is the mask of the IP above.
- **UDPPort** is the udp port of server which the communication
  packets should be send to.

### Communication
1. Encrypt packets from TUN and send them to server with UDP
2. Recieve and decrypt packets from UDP and send to TUN
Both packets are look like this:

```
+----------------------+
|    |  Encrypted Data |
| IV |-----------------|
|    |     Playload    |
|----+-----------------|
| 16 |     variable    |
+----------------------+
```
