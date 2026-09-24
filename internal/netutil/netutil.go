// Package netutil holds network helpers shared by the API client and the
// downloader.
package netutil

import (
	"errors"
	"net"
	"strings"
)

// Filtered reports whether err is a connection to one of the addresses that
// internet filtering in Iran answers for blocked sites (10.10.34.34-36).
// Such connections never succeed, so they are not worth retrying.
func Filtered(err error) bool {
	var op *net.OpError
	if !errors.As(err, &op) || op.Addr == nil {
		return false
	}
	host, _, splitErr := net.SplitHostPort(op.Addr.String())
	return splitErr == nil && strings.HasPrefix(host, "10.10.34.")
}
