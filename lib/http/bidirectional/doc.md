HTTP Bidirectional Tunnel
=========================

An HTTP bidirectional tunnel combines an HTTP server tunnel (forwarding incoming
I2P connections to a local HTTP service) with a SOCKS5 proxy client on the
**same I2P keys**. This allows a single tunnel to both:

1. Accept inbound I2P connections and forward them to a local HTTP service
2. Provide a SOCKS5 proxy for local applications to reach arbitrary I2P destinations

The tunnel uses a single `onramp.Garlic` instance, sharing the same I2P identity
(keys) for both directions.

Configuration type: `httpbidirectional`
