# socks
--
    import "github.com/go-i2p/go-i2ptunnel/lib/socks/client"


## Usage

#### type SOCKS

```go
type SOCKS struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// SOCKS5 server instance
	*socks5.Server

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}
```


#### func  NewSocksClient

```go
func NewSocksClient(config i2pconv.TunnelConfig, samAddr string) (*SOCKS, error)
```

#### func (*SOCKS) Address

```go
func (s *SOCKS) Address() string
```
Get the tunnel's I2P address

#### func (*SOCKS) Error

```go
func (s *SOCKS) Error() error
```
Get the tunnel's error message

#### func (*SOCKS) LocalAddress

```go
func (s *SOCKS) LocalAddress() (string, error)
```
Get the tunnel's local host:port

#### func (*SOCKS) Name

```go
func (s *SOCKS) Name() string
```
Get the tunnel's name

#### func (*SOCKS) Start

```go
func (s *SOCKS) Start() error
```
Start the tunnel

#### func (*SOCKS) Status

```go
func (s *SOCKS) Status() i2ptunnel.I2PTunnelStatus
```
Get the tunnel's status

#### func (*SOCKS) Stop

```go
func (s *SOCKS) Stop() error
```
Stop the tunnel

#### func (*SOCKS) TCPHandle

```go
func (s *SOCKS) TCPHandle(_ *socks5.Server, conn *net.TCPConn, req *socks5.Request) error
```
TCPHandle implements socks5.Handler.

#### func (*SOCKS) Target

```go
func (s *SOCKS) Target() string
```
Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5
and HTTP

#### func (*SOCKS) Type

```go
func (s *SOCKS) Type() string
```
Get the tunnel's type

#### func (*SOCKS) UDPHandle

```go
func (s *SOCKS) UDPHandle(_ *socks5.Server, addr *net.UDPAddr, data *socks5.Datagram) error
```
UDPHandle implements socks5.Handler.
