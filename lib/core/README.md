# I2PTunnel Core Interface

A Go package that defines the standard interface for I2P tunnel implementations. This core library provides the contract that all I2P tunnel types must implement to be compatible with I2PTunnel controllers.

## Features

- Standardized interface for I2P tunnel implementations
- Type-safe tunnel state management via constants
- Universal control methods for all tunnel types
- Common configuration and error handling patterns

## Installation

```bash
go get github.com/go-i2p/go-i2ptunnel/lib/core
```

## Usage Example

```go
// filepath: example/mytunnel.go
package mytunnel

import "github.com/go-i2p/go-i2ptunnel/lib/core"

type MyTunnel struct {
    name    string
    status  i2ptunnel.I2PTunnelStatus
    options map[string]string
}

// Implement the I2PTunnel interface
func (t *MyTunnel) Start() error {
    t.status = i2ptunnel.I2PTunnelStatusStarting
    // Your tunnel startup logic here
    return nil
}

func (t *MyTunnel) Status() i2ptunnel.I2PTunnelStatus {
    return t.status
}

// Implement remaining required methods...
```

## Required Interface Methods

All tunnel implementations must provide:

- `Start()` - Initialize and start the tunnel
- `Stop()` - Gracefully shutdown the tunnel
- `Name()` - Get tunnel identifier
- `Type()` - Get tunnel type
- `Address()` - Get I2P destination address
- `Target()` - Get target I2P address
- `Options()` - Get configuration options
- `Status()` - Get current state
- `Error()` - Get error state
- `LocalAddress()` - Get local endpoint

## Tunnel States

- `Running` - Active and operational
- `Stopped` - Inactive
- `Starting` - In startup process
- `Stopping` - In shutdown process
- `Failed` - Error state
- `Unknown` - Indeterminate state

## Testing

```bash
# Run unit tests
go test ./...

# Test your implementation
go test -v -run TestYourTunnel
```

## Acknowledgements

Based on the I2P project's tunnel specifications. Thanks to the I2P team and contributors.