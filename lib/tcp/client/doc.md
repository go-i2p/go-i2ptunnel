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
	// contains filtered or unexported fields
}
```

**Thread-safety**: The `errors` field is unexported and protected by an internal mutex.
Use `Error()` to retrieve the most recent error, or `ErrorHistory()` for a safe snapshot
of all recorded errors. Do **not** access unexported fields directly — concurrent reads
without a lock will be flagged by `go test -race`.


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
Get the tunnel's error message (most recent recorded error).

#### func (*TCPClient) ErrorHistory

```go
func (t *TCPClient) ErrorHistory() []i2ptunnel.I2PTunnelError
```
ErrorHistory returns a snapshot copy of all recorded errors, safe for concurrent use.
The returned slice is independent of the internal buffer — callers may iterate or store
it without holding any lock. Returns nil when no errors have been recorded.

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
