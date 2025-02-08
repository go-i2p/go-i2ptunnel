# tcpclient
--
    import "github.com/go-i2p/go-i2ptunnel/lib/tcp/client"


## Usage

#### type TCPClient

```go
type TCPClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The remote I2P destination target
	*i2pkeys.I2PAddr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}
```


#### func  NewTCPClient

```go
func NewTCPClient(config i2pconv.TunnelConfig, samAddr string) (*TCPClient, error)
```
NewTCPClient creates a new TCP Client tunnel with the given configuration

#### func (*TCPClient) Address

```go
func (t *TCPClient) Address() string
```
Get the tunnel's I2P address

#### func (*TCPClient) Error

```go
func (t *TCPClient) Error() error
```
Get the tunnel's error message

#### func (*TCPClient) LocalAddress

```go
func (t *TCPClient) LocalAddress() (string, error)
```
Get the tunnel's local host:port

#### func (*TCPClient) Name

```go
func (t *TCPClient) Name() string
```
Get the tunnel's name

#### func (*TCPClient) Options

```go
func (t *TCPClient) Options() map[string]string
```
Get the tunnel's options

#### func (*TCPClient) Start

```go
func (t *TCPClient) Start() error
```
Start the tunnel

#### func (*TCPClient) Status

```go
func (t *TCPClient) Status() i2ptunnel.I2PTunnelStatus
```
Get the tunnel's status

#### func (*TCPClient) Stop

```go
func (t *TCPClient) Stop() error
```
Stop the tunnel

#### func (*TCPClient) Target

```go
func (t *TCPClient) Target() string
```
Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5
and HTTP

#### func (*TCPClient) Type

```go
func (t *TCPClient) Type() string
```
Get the tunnel's type
