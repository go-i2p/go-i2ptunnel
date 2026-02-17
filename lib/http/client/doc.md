# httpclient
--
    import "github.com/go-i2p/go-i2ptunnel/lib/http/client"


## Usage

#### type HTTPClient

```go
type HTTPClient struct {
	// I2P Connection to listen to the I2P network
	*onramp.Garlic
	// The I2P Tunnel config itself
	i2pconv.TunnelConfig
	// The tunnel status
	i2ptunnel.I2PTunnelStatus
	// The http filtering configuration
	httpinspector.Config
	// The proxy server
	*goproxy.ProxyHttpServer
	// The http server
	*http.Server

	// Error history of the tunnel
	Errors []i2ptunnel.I2PTunnelError
}
```


#### func  NewHTTPClient

```go
func NewHTTPClient(config i2pconv.TunnelConfig, samAddr string) (*HTTPClient, error)
```
NewHTTPClient creates a new HTTP Client tunnel with the given configuration

#### func (*HTTPClient) Address

```go
func (h *HTTPClient) Address() string
```
Get the tunnel's I2P address

#### func (*HTTPClient) Dial

```go
func (h *HTTPClient) Dial(network, addr string) (c net.Conn, err error)
```

#### func (*HTTPClient) DialContext

```go
func (h *HTTPClient) DialContext(ctx context.Context, network, addr string) (c net.Conn, err error)
```

#### func (*HTTPClient) Error

```go
func (h *HTTPClient) Error() error
```
Get the tunnel's error message

#### func (*HTTPClient) LocalAddress

```go
func (h *HTTPClient) LocalAddress() (string, error)
```
Get the tunnel's local host:port

#### func (*HTTPClient) Name

```go
func (h *HTTPClient) Name() string
```
Get the tunnel's name

#### func (*HTTPClient) Start

```go
func (h *HTTPClient) Start() error
```
Start the tunnel

#### func (*HTTPClient) Status

```go
func (h *HTTPClient) Status() i2ptunnel.I2PTunnelStatus
```
Get the tunnel's status

#### func (*HTTPClient) Stop

```go
func (h *HTTPClient) Stop() error
```
Stop the tunnel

#### func (*HTTPClient) Target

```go
func (h *HTTPClient) Target() string
```
Get the tunnel's I2P target. Nil in the case of one-to-many clients like SOCKS5
and HTTP

#### func (*HTTPClient) Type

```go
func (h *HTTPClient) Type() string
```
Get the tunnel's type

#### type Outproxy

```go
type Outproxy struct {
	// Address is the I2P destination of the outproxy service.
	Address string
	// Enabled controls whether clearnet requests are forwarded.
	Enabled bool
}
```

Outproxy holds the configuration for routing clearnet requests through an I2P
outproxy service. When enabled, non-I2P HTTP requests are tunneled through the
specified I2P destination instead of being rejected.

#### func IsI2PAddress

```go
func IsI2PAddress(host string) bool
```

IsI2PAddress reports whether the given host is an I2P network address. Returns
true for any address ending in ".i2p".
