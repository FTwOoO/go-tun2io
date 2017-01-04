# go-tun2io

go-tun2io is similar to [tunio](https://github.com/getlantern/tunio) which based on [badvpn-tun2socks](https://github.com/ambrop72/badvpn)/lwip.

go-tun2io use [netstack](https://github.com/google/netstack) instead of badvpn-tun2socks/lwip as the userland tcpip stack, 
turn all TCP packets from network interface to SOCKS/HTTP proxy connection(as proxy.Dialer API).

## TCP Example

This example will access http://xahlee.info by this flow
(The default SOCKS5 server is hardcoded in main.go:`socksAddr = "52.69.162.110:1080"`):
```
TCP packets -> tun -> go-tun2io -> SOCKS5 server -> target(xahlee.info:80)
```


* create net interface with subnet 192.168.4.0/24

    ```
    ip tuntap add tun2 mode tun 
    ifconfig tun2 up
    ifconfig tun2 192.168.4.1 netmask 255.255.255.0
    
    ip route add 74.208.215.34 metric 1 dev tun2
    ip route add 0.0.0.0/0 metric 3 via 45.76.196.1 dev ens3
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

* run the example to access http://xahlee.info (ip=74.208.215.34)

    ```
    go run main.go tun2 192.168.4.1/24
    curl http://xahlee.info
    
    ```