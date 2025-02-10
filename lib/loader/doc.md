# loader
--
    import "github.com/go-i2p/go-i2ptunnel/lib/loader"


## Usage

```go
var DEFAULT_SAM_ADDRESS = "localhost:7656"
```

#### func  Load

```go
func Load(path string, samAddr ...string) (i2ptunnel.I2PTunnel, error)
```
using go-i2ptunnel-config's Converter, load the config file and return an
I2PTunnel matching the configured tunnel type
