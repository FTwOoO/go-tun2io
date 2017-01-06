# go-tun2io

go-tun2io is similar to [tunio](https://github.com/getlantern/tunio) which based on [badvpn-tun2socks](https://github.com/ambrop72/badvpn)/lwip.

go-tun2io use [netstack](https://github.com/google/netstack) instead of badvpn-tun2socks/lwip as the userland tcpip stack, 
turn all TCP packets from network interface to SOCKS/HTTP proxy connection(as proxy.Dialer API).

## Example

### TCP
The example will access http://xahlee.info webpage througth the tunnel:
```
TCP packets <-> tun <-> netstack <-> go-tun2io <-> SOCKS5 server <-> target(xahlee.info:80)
```

create net interface with ip 192.168.4.1/24:

    ```
    ip tuntap add tun2 mode tun 
    ifconfig tun2 up
    ifconfig tun2 192.168.4.1 netmask 255.255.255.0
    
    ip route add 74.208.215.34 metric 1 dev tun2
    ip route add 0.0.0.0/0 metric 2 via 45.76.196.1 dev ens3
    ip route delete 0.0.0.0/0 metric 0 via 45.76.196.1 dev ens3
    ```

    show the system routing table by runing `route -n`:

    ```
    Kernel IP routing table
    Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
    0.0.0.0         45.76.196.1     0.0.0.0         UG    2      0        0 ens3
    45.76.196.0     0.0.0.0         255.255.254.0   U     0      0        0 ens3
    169.254.169.254 45.76.196.1     255.255.255.255 UGH   0      0        0 ens3
    74.208.215.34   0.0.0.0         255.255.255.255 UH    1      0        0 tun2
    192.168.4.0     0.0.0.0         255.255.255.0   U     0      0        0 tun2
    ```

run the example to access http://xahlee.info (ip=74.208.215.34):

    ```
    go run test.go tun2 192.168.4.1/24 52.69.162.110:1080
    curl http://xahlee.info
    
    ```
    
### UDP
The example will send DNS request for domain `xahlee.info` to Google DNS Server(8.8.8.8) and get the DNS response:

```
DNS request <-> netstack <-> go-tun2io <-> target(8.8.8.8:53)
```

The DNS request is injected directly inoto `netstack`, so tcpdump can not cature the DNS request, but 
you can cature the DNS response:

    ```
    tcpdump -i tun2 -vvv -n
    
    tcpdump: listening on tun2, link-type RAW (Raw IP), capture size 262144 bytes
    12:12:40.821559 IP (tos 0x0, ttl 64, id 51067, offset 0, flags [DF], proto TCP (6), length 40)
    192.168.4.1.54518 > 74.208.215.34.80: Flags [F.], cksum 0x49ee (correct), seq 2804701053, ack 1545723174, win 29200, length 0
    ```