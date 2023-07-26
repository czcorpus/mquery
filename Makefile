all: test build

build:
	manabuild mquery

install:
	cp ./mquery /usr/local/bin

clean:
	rm mquery

test:
	go test ./...

rtest:
	go test -race ./...

.PHONY: clean install test