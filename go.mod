module github.com/go-i2p/go-i2ptunnel

go 1.26.3

require (
	github.com/elazarl/goproxy v1.9.1
	github.com/go-i2p/go-connfilter v0.0.0-20260908205212-5a1da11e5bd8
	github.com/go-i2p/go-forward v0.0.0-20260908205159-b32a1ef9df93
	github.com/go-i2p/go-i2ptunnel-config v0.0.0-20260908204747-53f201ef33c4
	github.com/go-i2p/go-limit v0.0.0-20260908193410-36af9901aa80
	github.com/go-i2p/go-sam-go v0.33.1
	github.com/go-i2p/i2pkeys v0.33.92
	github.com/go-i2p/logger v0.1.60000-0.20260701134448-2648c3b0e040
	//github.com/go-i2p/onramp b7b6382ac96a5f9fed09c7adfd8f549d571f642b
	github.com/txthinking/socks5 v0.0.0-20260601051520-339b044ab0eb
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/go-i2p/onramp v0.0.0-00010101000000-000000000000
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
	github.com/go-i2p/common v0.1.60000-0.20260701134558-e5f5cf65a7f5 // indirect
	github.com/go-i2p/crypto v0.1.60000-0.20260701135847-3ade996b68a0 // indirect
	github.com/go-i2p/elgamal v0.1.60000-0.20260701131626-b5c8141026fc // indirect
	github.com/go-i2p/go-datagrams v0.1.67 // indirect
	github.com/go-i2p/go-i2cp v0.1.60000-0.20260701134816-aa86eb2db4a5 // indirect
	github.com/go-i2p/go-i2p v0.1.67 // indirect
	github.com/go-i2p/go-nat-listener v0.1.68 // indirect
	github.com/go-i2p/go-noise v0.1.60000-0.20260701134708-a15c74fe448e // indirect
	github.com/go-i2p/go-sam-bridge v0.1.67 // indirect
	github.com/go-i2p/go-streaming v0.1.67 // indirect
	github.com/go-i2p/go-unzip v0.0.0-20260908193701-6ecb33740b07 // indirect
	github.com/go-i2p/noise v1.1.1-0.20260612013029-6d1ef3427561 // indirect
	github.com/go-i2p/path v0.1.60000-0.20260701134624-ee3ce0cb2483 // indirect
	github.com/go-i2p/pool v0.1.60000-0.20260701134610-0724f29e205d // indirect
	github.com/go-i2p/red25519 v0.0.0-20260908204607-9444de34c638 // indirect
	github.com/go-i2p/su3 v0.1.60000-0.20260701134531-62efdb4e3f71 // indirect
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
