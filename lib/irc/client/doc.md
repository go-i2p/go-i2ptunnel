# ircclient
--
    import "github.com/go-i2p/go-i2ptunnel/lib/irc/client"


## Usage

#### type IRCClient

```go
type IRCClient struct {
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


#### func  NewIRCClient

```go
func NewIRCClient(config i2pconv.TunnelConfig, samAddr string) (*IRCClient, error)
```
NewIRCClient creates a new IRC Client tunnel with the given configuration

#### func (*IRCClient) Address

```go
func (i *IRCClient) Address() string
```
Get the tunnel's I2P address

#### func (*IRCClient) Error

```go
func (i *IRCClient) Error() error
```
Get the tunnel's error message

#### func (*IRCClient) LocalAddress

```go
func (i *IRCClient) LocalAddress() (string, string, error)
```
Get the tunnel's local host:port

#### func (*IRCClient) Name

```go
func (i *IRCClient) Name() string
```
Get the tunnel's name

#### func (*IRCClient) Options

```go
func (i *IRCClient) Options() map[string]string
```
Get the tunnel's options

#### func (*IRCClient) Start

```go
func (i *IRCClient) Start() error
```
Start the tunnel

#### func (*IRCClient) Status

```go
func (i *IRCClient) Status() i2ptunnel.I2PTunnelStatus
```
Get the tunnel's status

#### func (*IRCClient) Stop

```go
func (i *IRCClient) Stop() error
```
Stop the tunnel

#### func (*IRCClient) Target

```go
func (i *IRCClient) Target() string
```
Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5
and HTTP

#### func (*IRCClient) Type

```go
func (i *IRCClient) Type() string
```
Get the tunnel's type
