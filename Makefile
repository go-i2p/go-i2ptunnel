HERE=$(shell pwd)

fmt:
	find . -name '*.go' -exec gofumpt -w -s -extra {} \;

doc:
	find lib cmd -type d -exec $(HERE)/doc.sh {} \;