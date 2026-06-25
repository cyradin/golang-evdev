.PHONY: ecodes lint test

HEADERS  = /usr/include/linux/input.h
HEADERS += /usr/include/linux/input-event-codes.h

ecodes:
	go run github.com/cyradin/golang-evdev/cmd/ecodes $(HEADERS) > ecodes.go
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4 run
test:
	go test ./... --race