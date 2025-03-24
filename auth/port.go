package auth

import (
	"fmt"
	"net"
)

func TestPort80() error {
	// sudo setcap CAP_NET_BIND_SERVICE=+eip /home/steven/go/bin/osrs-launcher
	l, err := net.Listen("tcp", ":80")
	if err != nil {
		return fmt.Errorf("cannot list on port 80, auth will fail: %w", err)
	}

	return l.Close()
}
