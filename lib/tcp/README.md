TCP "Standard" Tunnels:
=======================

TCP Tunnels are also known as "Standard" tunnels, and are used to connect TCP clients and services to I2P.
These tunnels do no filtering, and pass traffic unmodified end-to-end.

TCP Server Tunnel
-----------------

The TCP Server tunnel consists of a TCP Client which connects to an external service operating on a local port,
and an I2P Service which listens on an I2P Destination and accepts connections from clients inside I2P.
When the I2P Service recieves a connection, it uses the TCP client to forward the connection to the external service.
When the external service replies, the response is sent back down the I2P tunnel back to the client.
This is a point-to-point connection.

TCP Client Tunnel
-----------------

The TCP Client tunnel consists of a TCP Server which listens on a local port,
and an I2P Client which connects to a single destination.
When the TCP Server recieves a connection, it forwards the connection to the I2P client who sends it to the I2P Destination.
This is a point-to-point connection.