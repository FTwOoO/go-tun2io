#!/usr/bin/env python3
# -*- coding: utf-8 -*-
#
#
#
#

from scapy.all import *
import os
from pytun import TunTapDevice

TUN_NAME='tun2'

TUNSETIFF = 0x400454ca
IFF_TUN = 0x0001
IFF_TAP = 0x0002
TUNMODE = IFF_TUN

tun = TunTapDevice(name=TUN_NAME)
#f = os.open("/dev/net/tun", os.O_RDWR)
#_ = ioctl(f, TUNSETIFF, struct.pack("16sH", TUN_NAME.encode('utf-8'), TUNMODE))

# Speed optimization so Scapy does not have to parse payloads
Ether.payload_guess = []

n = 10

while n>0:
    packet = IP(src="192.168.1.1", dst="xahlee.info") / TCP(dport=[80])/b"GET / HTTP/1.0\r\n\r\n"
    #os.write(f, packet)
    tun.write(bytes(packet))
    n -= 1
