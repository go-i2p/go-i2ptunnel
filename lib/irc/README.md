IRC Tunnels
============

The go-i2ptunnel IRC suite provides a secure tunneling proxy system for connecting IRC clients and servers between the I2P network and local services. It consists of both client and server components that handle protocol-specific requirements for IRC communications over I2P, while maintaining security and privacy.

IRC Client
----------

The IRC Client implements a SOCKS-compatible proxy that enables local IRC clients to connect to services on the I2P network. It provides:

- Transparent proxying between local IRC clients and I2P servers
- Command filtering for enhanced security
- Connection management and automatic reconnection
- Bandwidth usage monitoring

IRC Server
----------

The IRC Server implements a reverse proxy that enables IRC servers hosted on the local machine to be accessible from the I2P network. It provides:

- Secure traffic forwarding between local IRC services and I2P clients
- Access control and connection management
- Command filtering and security policies
- Bandwidth and resource monitoring
