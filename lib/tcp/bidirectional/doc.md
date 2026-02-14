TCP Bidirectional Tunnel
========================

A TCP bidirectional tunnel combines a TCP server tunnel (forwarding incoming I2P
connections to a local service) with a SOCKS5 proxy client on the **same I2P
keys**. This allows a single tunnel to both:

1. Accept inbound I2P connections and forward them to a local TCP service
2. Provide a SOCKS5 proxy for local applications to reach arbitrary I2P destinations

Traffic flows:

- **Inbound:** I2P Network → I2P Listener → TCP Client → Local Service
- **Outbound:** Local App → SOCKS5 Proxy → I2P Client → I2P Network → Destination

The tunnel uses a single `onramp.Garlic` instance, sharing the same I2P identity
(keys) for both directions. The SOCKS5 proxy runs on a separate local port from
the I2P listener.

Configuration type: `tcpbidirectional`
