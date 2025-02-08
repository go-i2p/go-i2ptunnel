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

#### func (*HTTPClient) Options

```go
func (h *HTTPClient) Options() map[string]string
```
Get the tunnel's options

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
