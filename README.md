# go-tun2io
 A project turns tcp packets from network interface into proxy.Dialer.

go-tun2io is similar to [tunio](https://github.com/getlantern/tunio) which based on [badvpn-tun2socks](https://github.com/ambrop72/badvpn)/lwip.

go-tun2io use [netstack](https://github.com/google/netstack) instead of badvpn-tun2socks/lwip as the userland tcpip stack, 
turn all TCP packets from network interface to SOCKS/HTTP proxy connection(as proxy.Dialer API).

## Example
The test.go create the tun device(with ip `192.168.4.1/24` and name `tun2`) and run the go-tun2io 
with SOCKS5 server `52.69.162.110:1080`, 

Create a tun interface with ip `192.168.4.1/24`, `74.208.215.34` is the ip of domain `xahlee.info`, a target for 
the following TCP test, so we route it through `tun2`:

    ```
    ip tuntap add tun2 mode tun 
    ifconfig tun2 up
    ifconfig tun2 192.168.4.1 netmask 255.255.255.0
    
    ip route add 74.208.215.34 metric 1 dev tun2
    ip route add 0.0.0.0/0 metric 2 via 45.76.196.1 dev ens3
    ip route delete 0.0.0.0/0 metric 0 via 45.76.196.1 dev ens3
    ```

Show the system routing table by runing `route -n`:

    ```
    Kernel IP routing table
    Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
    0.0.0.0         45.76.196.1     0.0.0.0         UG    2      0        0 ens3
    45.76.196.0     0.0.0.0         255.255.254.0   U     0      0        0 ens3
    169.254.169.254 45.76.196.1     255.255.255.255 UGH   0      0        0 ens3
    74.208.215.34   0.0.0.0         255.255.255.255 UH    1      0        0 tun2
    192.168.4.0     0.0.0.0         255.255.255.0   U     0      0        0 tun2
    ```

Run the example:

    ```
    go run test.go
    
    ```
    
### TCP
The example will access http://xahlee.info blog:

    ```
    curl http://xahlee.info
    ```
    
Because the ip`74.208.215.34` of domain `xahlee.info` routes through `tun2` interface, 
so the TCP packets of the HTTP session will go througth the TCP tunnel:

```
TCP packets <-> tun <-> netstack <-> go-tun2io <--tunnel--> SOCKS5 server <-> target(xahlee.info:80)
```


### UDP
The example will send a DNS request asking for domain `facebook.com` to the 
Google DNS Server `8.8.8.8:53`,  the UDP packets will go through the UDP tunnel to get response:

```
UDP packets(DNS request) <-> netstack <-> go-tun2io <--tunnel--> target(8.8.8.8:53)
```

The DNS request is injected directly into `netstack`, so tcpdump can not cature the DNS request, but 
you can cature the DNS response:

    ```
    tcpdump -i tun2 -vvv -n
    
    05:28:31.761613 IP (tos 0x0, ttl 65, id 44388, offset 0, flags [none], proto UDP (17), length 74)
    8.8.8.8.53 > 192.168.4.1.10078: [udp sum ok] 58680 q: A? facebook.com. 1/0/0 facebook.com. A 31.13.95.36 (46)
    ```
    
### Local DNS Server
The example create a local UDP endpoint listenning on `192.168.4.1:53` for DNS request,
if received requests, forward them to the real remote DNS server `8.8.8.8:53`, and then get back 
the response:

```
DNS request <-> netstack <-> go-tun2io <--forward--> target(8.8.8.8:53)
```


The DNS request is injected directly into `netstack`, so tcpdump can not cature the DNS request, but 
you can cature the DNS response:

    ```    
    05:28:31.763979 IP (tos 0x0, ttl 65, id 32268, offset 0, flags [none], proto UDP (17), length 111)
    192.168.4.1.53 > 192.168.4.1.10079: [udp sum ok] 6519 q: A? twitter.com. 2/0/0 twitter.com. A 104.244.42.1, twitter.com. A 104.244.42.129 (83)
    ```
