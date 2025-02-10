# httpserver
--
    import "github.com/go-i2p/go-i2ptunnel/lib/http/server"


## Usage

#### type HTTPServer

```go
type HTTPServer struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The local HTTP service address
	net.Addr
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The rate-limiting configuration
	limitedlistener.LimitedConfig
	// The http filtering configuration
	httpinspector.Config

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}
```


#### func  NewHTTPServer

```go
func NewHTTPServer(config i2pconv.TunnelConfig, samAddr string) (*HTTPServer, error)
```
NewHTTPServer creates a new HTTP Server tunnel with the given configuration

#### func (*HTTPServer) Address

```go
func (h *HTTPServer) Address() string
```
Get the tunnel's I2P address

#### func (*HTTPServer) Error

```go
func (h *HTTPServer) Error() error
```
Get the tunnel's error message

#### func (*HTTPServer) LocalAddress

```go
func (h *HTTPServer) LocalAddress() (string, error)
```
Get the tunnel's local host:port

#### func (*HTTPServer) Name

```go
func (h *HTTPServer) Name() string
```
Get the tunnel's name

#### func (*HTTPServer) Start

```go
func (h *HTTPServer) Start() error
```
Start the tunnel

#### func (*HTTPServer) Status

```go
func (h *HTTPServer) Status() i2ptunnel.I2PTunnelStatus
```
Get the tunnel's status

#### func (*HTTPServer) Stop

```go
func (h *HTTPServer) Stop() error
```
Stop the tunnel

#### func (*HTTPServer) Target

```go
func (h *HTTPServer) Target() string
```
Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5
and HTTP

#### func (*HTTPServer) Type

```go
func (h *HTTPServer) Type() string
```
Get the tunnel's type
