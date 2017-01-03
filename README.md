# go-tun2io

go-tun2io is similar to [tunio](https://github.com/getlantern/tunio) which based on [badvpn-tun2socks](https://github.com/ambrop72/badvpn)/lwip.

go-tun2io use [netstack](https://github.com/google/netstack) instead of badvpn-tun2socks/lwip as the userland tcpip stack, 
turn all TCP packets from network interface to SOCKS/HTTP proxy connection(as proxy.Dialer API).
