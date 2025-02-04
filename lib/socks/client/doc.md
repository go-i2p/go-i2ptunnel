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

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}
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
func (s *SOCKS) LocalAddress() (string, string, error)
```
Get the tunnel's local host:port

#### func (*SOCKS) Name

```go
func (s *SOCKS) Name() string
```
Get the tunnel's name

#### func (*SOCKS) Options

```go
func (s *SOCKS) Options() map[string]string
```
Get the tunnel's options

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
