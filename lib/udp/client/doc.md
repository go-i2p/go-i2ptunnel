# udpclient
--
    import "github.com/go-i2p/go-i2ptunnel/lib/udp/client"


## Usage

#### type UDPClient

```go
type UDPClient struct {
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


#### func  NewUDPClient

```go
func NewUDPClient(config i2pconv.TunnelConfig, samAddr string) (*UDPClient, error)
```
NewUDPClient creates a new UDP Client tunnel with the given configuration

#### func (*UDPClient) Address

```go
func (u *UDPClient) Address() string
```
Get the tunnel's I2P address

#### func (*UDPClient) Error

```go
func (u *UDPClient) Error() error
```
Get the tunnel's error message

#### func (*UDPClient) LocalAddress

```go
func (u *UDPClient) LocalAddress() (string, error)
```
Get the tunnel's local host:port

#### func (*UDPClient) Name

```go
func (u *UDPClient) Name() string
```
Get the tunnel's name

#### func (*UDPClient) Start

```go
func (u *UDPClient) Start() error
```
Start the tunnel

#### func (*UDPClient) Status

```go
func (u *UDPClient) Status() i2ptunnel.I2PTunnelStatus
```
Get the tunnel's status

#### func (*UDPClient) Stop

```go
func (u *UDPClient) Stop() error
```
Stop the tunnel

#### func (*UDPClient) Target

```go
func (u *UDPClient) Target() string
```
Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5
and HTTP

#### func (*UDPClient) Type

```go
func (u *UDPClient) Type() string
```
Get the tunnel's type
