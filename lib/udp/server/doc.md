# udpserver
--
    import "github.com/go-i2p/go-i2ptunnel/lib/udp/server"


## Usage

#### type UDPServer

```go
type UDPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local UDP service address
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}
```


#### func  NewUDPServer

```go
func NewUDPServer(config i2pconv.TunnelConfig, samAddr string) (*UDPServer, error)
```
NewUDPServer creates a new UDP Server tunnel with the given configuration

#### func (*UDPServer) Address

```go
func (u *UDPServer) Address() string
```
Get the tunnel's I2P address

#### func (*UDPServer) Error

```go
func (u *UDPServer) Error() error
```
Get the tunnel's error message

#### func (*UDPServer) LocalAddress

```go
func (u *UDPServer) LocalAddress() (string, error)
```
Get the tunnel's local host:port

#### func (*UDPServer) Name

```go
func (u *UDPServer) Name() string
```
Get the tunnel's name

#### func (*UDPServer) Options

```go
func (u *UDPServer) Options() map[string]string
```
Get the tunnel's options

#### func (*UDPServer) Start

```go
func (u *UDPServer) Start() error
```
Start the tunnel

#### func (*UDPServer) Status

```go
func (u *UDPServer) Status() i2ptunnel.I2PTunnelStatus
```
Get the tunnel's status

#### func (*UDPServer) Stop

```go
func (u *UDPServer) Stop() error
```
Stop the tunnel

#### func (*UDPServer) Target

```go
func (u *UDPServer) Target() string
```
Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5
and HTTP

#### func (*UDPServer) Type

```go
func (u *UDPServer) Type() string
```
Get the tunnel's type
