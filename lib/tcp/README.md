TCP "Standard" Tunnels:
=======================

The TCP tunnels proxy suite provides transparent bridging between TCP applications and I2P networks. It implements two core tunnel types:

1. TCP Server tunnels - Allow I2P clients to connect to local TCP services
2. TCP Client tunnels - Enable local TCP clients to access I2P services

These "Standard" tunnels create secure, anonymous communication channels by establishing direct connections between TCP endpoints and I2P destinations, while maintaining full TCP protocol compatibility.

TCP Server Tunnel
-----------------

A TCP Server tunnel connects a local TCP service to the I2P network through:
1. A TCP Client component that interfaces with the local service
2. An I2P Service component that maintains a persistent destination address

When an I2P peer connects to the tunnel's destination, the traffic flows:
- Incoming: I2P Network → I2P Service → TCP Client → Local Service
- Outgoing: Local Service → TCP Client → I2P Service → I2P Network

TCP Client Tunnel
-----------------

A TCP Client tunnel operates by:
1. Running a TCP Server that listens on a local port
2. Maintaining an I2P Client connected to a specific destination

When activated:
- Local applications connect to the TCP Server
- Traffic routes through the I2P Client to the target I2P destination
- Creates a secure point-to-point connection

Both tunnel types preserve the original TCP traffic while adding I2P's anonymity and encryption layers.

When a local client connects to the I2P tunnel's destination, the traffic flows:
- Outgoing: Local Client → TCP Server → I2P Client → I2P Network
- Incoming: I2P Network → I2P Client → TCP Server → Local Client