module github.com/go-i2p/go-i2ptunnel

go 1.24.2

toolchain go1.24.4

require (
	github.com/elazarl/goproxy v1.7.0
	github.com/go-i2p/go-connfilter v0.0.0-20250205023438-0f2b889a80f6
	github.com/go-i2p/go-forward v0.0.0-20250202052226-ee8a43dcb664
	github.com/go-i2p/go-i2ptunnel-config v0.0.0-20251019021515-10ef90f2473b
	github.com/go-i2p/go-limit v0.0.0-20250203203118-210616857c15
	github.com/go-i2p/go-sam-go v0.0.0-20251019222649-156dfde398c8
	github.com/go-i2p/i2pkeys v0.33.92
	github.com/go-i2p/logger v0.0.0-20251021001827-b7cf9a3377d9
	github.com/go-i2p/onramp v0.33.93-0.20251019222841-39bbd6584c39
	github.com/txthinking/socks5 v0.0.0-20230325130024-4230056ae301
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/cretz/bine v0.2.0 // indirect
	github.com/go-i2p/common v0.0.0-20250819203334-e5459df35789 // indirect
	github.com/go-i2p/crypto v0.0.0-20250822224541-85015740db11 // indirect
	github.com/magiconair/properties v1.8.9 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/samber/lo v1.51.0 // indirect
	github.com/samber/oops v1.19.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/txthinking/runnergroup v0.0.0-20210608031112-152c7c4432bf // indirect
	github.com/urfave/cli v1.22.16 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/time v0.9.0 // indirect
)

replace github.com/go-i2p/go-i2ptunnel-config => ../go-i2ptunnel-config

replace github.com/go-i2p/onramp => ../onramp

replace github.com/go-i2p/go-sam-go => ../go-sam-go

replace github.com/go-i2p/go-connfilter => ../go-connfilter

replace github.com/go-i2p/go-forward => ../go-forward

replace github.com/go-i2p/go-limit => ../go-limit

//replace github.com/go-i2p/sam3 => ../sam3

replace github.com/go-i2p/logger => ../logger

//replace github.com/go-i2p/i2pkeys => ../i2pkeys
