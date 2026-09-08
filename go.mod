module github.com/go-i2p/go-i2ptunnel

go 1.26.3

require (
	github.com/elazarl/goproxy v1.9.1
	github.com/go-i2p/go-connfilter v0.0.0-20260908193148-911555473c45
	github.com/go-i2p/go-forward v0.0.0-20260908191756-4485ca6c256d
	github.com/go-i2p/go-i2ptunnel-config v0.0.0-20260908192949-b2a3b7d40e6e
	github.com/go-i2p/go-limit v0.0.0-20260908191839-5f71073a97f0
	github.com/go-i2p/go-sam-go v0.33.1
	github.com/go-i2p/i2pkeys v0.33.92
	github.com/go-i2p/logger v0.1.60000-0.20260701134448-2648c3b0e040
	//github.com/go-i2p/onramp b7b6382ac96a5f9fed09c7adfd8f549d571f642b
	github.com/txthinking/socks5 v0.0.0-20260601051520-339b044ab0eb
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/go-i2p/onramp v0.33.93-0.20260424200814-b7b6382ac96a
	golang.org/x/time v0.16.0
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2 // indirect
	github.com/beevik/ntp v1.5.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cretz/bine v0.2.0 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-i2p/common v0.1.60000-0.20260612012818-ffa84716e922 // indirect
	github.com/go-i2p/crypto v0.1.60000-0.20260612012729-bcf561943886 // indirect
	github.com/go-i2p/elgamal v0.1.59999 // indirect
	github.com/go-i2p/go-datagrams v0.1.59999 // indirect
	github.com/go-i2p/go-i2cp v0.1.60000-0.20260612013604-6be0e753d2c7 // indirect
	github.com/go-i2p/go-i2p v0.1.59999 // indirect
	github.com/go-i2p/go-nat-listener v0.1.68 // indirect
	github.com/go-i2p/go-noise v0.1.60000-0.20260624210455-c5cd198c4109 // indirect
	github.com/go-i2p/go-sam-bridge v0.1.59999 // indirect
	github.com/go-i2p/go-streaming v0.1.59999 // indirect
	github.com/go-i2p/go-unzip v0.0.0-20260908192035-1d47af1fc238 // indirect
	github.com/go-i2p/noise v1.1.1-0.20260612013029-6d1ef3427561 // indirect
	github.com/go-i2p/path v0.1.60000-0.20260612012938-437d7ee74b3a // indirect
	github.com/go-i2p/pool v0.1.59999 // indirect
	github.com/go-i2p/red25519 v0.0.0-20260908192929-b906f5fda5c0 // indirect
	github.com/go-i2p/su3 v0.1.60000-0.20260612012616-389e1d2c2310 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/huin/goupnp v1.3.0 // indirect
	github.com/jackpal/go-nat-pmp v1.1.0 // indirect
	github.com/magiconair/properties v1.18.11 // indirect
	github.com/oklog/ulid/v2 v2.1.2 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/samber/lo v1.53.0 // indirect
	github.com/samber/oops v1.23.1 // indirect
	github.com/sirupsen/logrus v1.10.2 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/txthinking/runnergroup v0.0.0-20250224021307-5864ffeb65ae // indirect
	github.com/urfave/cli/v2 v2.27.7 // indirect
	github.com/xrash/smetrics v0.0.0-20250705151800-55b8f293f342 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.step.sm/crypto v0.90.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

//replace github.com/go-i2p/go-i2ptunnel-config => ../go-i2ptunnel-config

replace github.com/go-i2p/onramp => ../onramp

//replace github.com/go-i2p/go-sam-go => ../go-sam-go

//replace github.com/go-i2p/go-connfilter => ../go-connfilter

//replace github.com/go-i2p/go-forward => ../go-forward

//replace github.com/go-i2p/go-limit => ../go-limit

//replace github.com/go-i2p/sam3 => ../sam3

//replace github.com/go-i2p/logger => ../logger

//replace github.com/go-i2p/i2pkeys => ../i2pkeys

retract (
	v0.1.59999
	v0.1.5999
	v0.1.599
)
