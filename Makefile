.PHONY: ecodes

HEADERS  = /usr/include/linux/input.h
HEADERS += /usr/include/linux/input-event-codes.h

ecodes:
	go run github.com/cyradin/golang-evdev/cmd/ecodes $(HEADERS) > ecodes.go

