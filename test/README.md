##Goal
  The test would run **myvpn** in a simulated network created with mininet running in Docker container. The purpose of this approach is to test the program in a sandbox, changable networking environment.
  The topology of the network is:
 
  
     10.0.1.100                                                    10.0.3.100
     ==========                 ===============                   ===========
     |        |                 |             |10.0.3.1           |         |
     | client |-------s1--------|eth1 r0  eth3|----------s2-------|  server |
     |        |        10.0.1.1 |             |                   |         |
     |        |                 |    eth2     |                   |         |
     ==========                 ===============                   ===========
                                      |10.0.5.1                
                                      |                            10.0.5.100
                                      |                           =============
                                      |                           |           |
                                      -------------s3-------------|   target  | 
                                                                  |           |
                                                                  =============
##Dependency
  The test program relies on:
  * Docker
  * Wireshark
  * Only supports Linux operating system.

##How to run
  In the test directory, please follow below steps:
  * $make build
  * $make run    
  * You should be able to see both wireshark and a mininet console. In the mininet console, run below commands to start **myvpn**:
    * mininet>server ./tinyvpn/server-start.sh
    * mininet>client ./tinyvpn/client-start.sh
    * You should be able to observe the connection established.
  * The network qos can be adjusted in the Makefile, by changing $(CMD_START_MN) with qos settings.

##How to observe
  * Open another console of the mininet container
    * $docker attach tinyvpn_mininet
  * Observe the network status of each host
    * mininet>client ip route
    * mininet>client ifconfig -a
  * You can filter the packets in Wireshark by adding filers.
    For example:
    ip.src == 10.0.1.100
  

