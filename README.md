# go-tun2io

go-tun2io forwards TCP packets to a net.Dialer, similar to [tunio](https://github.com/getlantern/tunio) and [badvpn-tun2socks](https://github.com/ambrop72/badvpn), but use [netstack](https://github.com/google/netstack) instead of badvpn-tun2socks(based on [lwip](http://savannah.nongnu.org/projects/lwip/)) for tcp packet processing.
