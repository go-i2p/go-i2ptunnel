# ircserver
--
    import "github.com/go-i2p/go-i2ptunnel/lib/irc/server"


## Usage

#### type IRCServer

```go
type IRCServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local IRC service address
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration
	limitedlistener.LimitedConfig
	// The IRC filtering configuration
	ircinspector.Config

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}
```


#### func  NewIRCServer

```go
func NewIRCServer(config i2pconv.TunnelConfig, samAddr string) (*IRCServer, error)
```
NewIRCServer creates a new IRC Server tunnel with the given configuration

#### func (*IRCServer) Address

```go
func (i *IRCServer) Address() string
```
Get the tunnel's I2P address

#### func (*IRCServer) Error

```go
func (i *IRCServer) Error() error
```
Get the tunnel's error message

#### func (*IRCServer) LocalAddress

```go
func (i *IRCServer) LocalAddress() (string, error)
```
Get the tunnel's local host:port

#### func (*IRCServer) Name

```go
func (i *IRCServer) Name() string
```
Get the tunnel's name

#### func (*IRCServer) Options

```go
func (i *IRCServer) Options() map[string]string
```
Get the tunnel's options

#### func (*IRCServer) Start

```go
func (i *IRCServer) Start() error
```
Start the tunnel

#### func (*IRCServer) Status

```go
func (i *IRCServer) Status() i2ptunnel.I2PTunnelStatus
```
Get the tunnel's status

#### func (*IRCServer) Stop

```go
func (i *IRCServer) Stop() error
```
Stop the tunnel

#### func (*IRCServer) Target

```go
func (i *IRCServer) Target() string
```
Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5
and HTTP

#### func (*IRCServer) Type

```go
func (i *IRCServer) Type() string
```
Get the tunnel's type
