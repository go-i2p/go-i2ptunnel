UDP Bidirectional Tunnel
========================

A UDP bidirectional tunnel combines a UDP server tunnel (forwarding incoming I2P
datagrams to a local service) with a SOCKS5 proxy client on the **same I2P
keys**. This allows a single tunnel to both:

1. Accept inbound I2P datagrams and forward them to a local UDP service
2. Provide a SOCKS5 proxy for local applications to reach arbitrary I2P destinations

This is a non-standard mode that uses onramp's capabilities to handle both
stream and datagram traffic on the same I2P identity.

Configuration type: `udpbidirectional`
