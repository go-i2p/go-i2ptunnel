HERE=$(shell pwd)

bin: fmt
	go build ./...

fmt:
	find . -name '*.go' -exec gofumpt -w -s -extra {} \;

doc: checklist
	find lib cmd -type d -exec $(HERE)/doc.sh {} \;

checklist:
	find . -name '*.go' -exec grep --color -C 1 -Hn 'panic("unimplemented")' {} \;