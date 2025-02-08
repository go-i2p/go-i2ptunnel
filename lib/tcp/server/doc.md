# tcpserver
--
    import "github.com/go-i2p/go-i2ptunnel/lib/tcp/server"


## Usage

#### type TCPServer

```go
type TCPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local TCP service address
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration
	limitedlistener.LimitedConfig

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}
```


#### func  NewTCPServer

```go
func NewTCPServer(config i2pconv.TunnelConfig, samAddr string) (*TCPServer, error)
```
NewTCPServer creates a new TCP Server tunnel with the given configuration

#### func (*TCPServer) Address

```go
func (t *TCPServer) Address() string
```
Get the tunnel's I2P address

#### func (*TCPServer) Error

```go
func (t *TCPServer) Error() error
```
Get the tunnel's error message

#### func (*TCPServer) LocalAddress

```go
func (t *TCPServer) LocalAddress() (string, error)
```
Get the tunnel's local host:port

#### func (*TCPServer) Name

```go
func (t *TCPServer) Name() string
```
Get the tunnel's name

#### func (*TCPServer) Options

```go
func (t *TCPServer) Options() map[string]string
```
Get the tunnel's options

#### func (*TCPServer) Start

```go
func (t *TCPServer) Start() error
```
Start the tunnel

#### func (*TCPServer) Status

```go
func (t *TCPServer) Status() i2ptunnel.I2PTunnelStatus
```
Get the tunnel's status

#### func (*TCPServer) Stop

```go
func (t *TCPServer) Stop() error
```
Stop the tunnel

#### func (*TCPServer) Target

```go
func (t *TCPServer) Target() string
```
Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5
and HTTP

#### func (*TCPServer) Type

```go
func (t *TCPServer) Type() string
```
Get the tunnel's type
