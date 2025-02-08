HERE=$(shell pwd)

bin: fmt
	go build ./...

fmt:
	find . -name '*.go' -exec gofumpt -w -s -extra {} \;

doc: checklist
	find lib cmd -type d -exec $(HERE)/doc.sh {} \;

checklist:
	find . -name '*.go' -exec grep --color -C 1 -Hn 'panic("unimplemented")' {} \; 2> /dev/null 1> UNIMPLEMENTED.md

mobile:
	go install golang.org/x/mobile/cmd/gomobile@latest
	gomobile init

bindSetupMobile: mobile
	go get -u golang.org/x/mobile/bind

mobileLibs:
	gomobile bind -target=android github.com/go-i2p/go-i2ptunnel/lib/tcp/client
	gomobile bind -target=android github.com/go-i2p/go-i2ptunnel/lib/tcp/server
	gomobile bind -target=android github.com/go-i2p/go-i2ptunnel/lib/udp/client
	gomobile bind -target=android github.com/go-i2p/go-i2ptunnel/lib/udp/server
	gomobile bind -target=android github.com/go-i2p/go-i2ptunnel/lib/irc/client
	gomobile bind -target=android github.com/go-i2p/go-i2ptunnel/lib/irc/server
	gomobile bind -target=android github.com/go-i2p/go-i2ptunnel/lib/http/client
	gomobile bind -target=android github.com/go-i2p/go-i2ptunnel/lib/http/server
	gomobile bind -target=android github.com/go-i2p/go-i2ptunnel/lib/socks/client