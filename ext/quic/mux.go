package quic

import (
	"log"
	"net/http"

	"github.com/costinm/ugate"
	"github.com/lucas-clemente/quic-go"
)

// QuicMUX is a mux to a specific node. May be accepted or dialed.
// Equivalent with quic/h3/client.go ( when dialing ), and Quic server when accept.
type QuicMUX struct {
	// Remote - has an ID and Addr
	n *ugate.DMNode

	// Can be accepted or dialed - the protocol is symmetric.
	s quic.EarlySession

	// quic/http3 client - currently used for sending the headers.
	// It is not the most efficient - for TCP we can avoid the extra DATA framing
	// and use simpler headers.
	rt            http.RoundTripper

	// required by client - there is an extra check on the URL
	hostname string
	client bool
}

func (ugs *QuicMUX) Close() error {
	if ugate.DebugClose {
		log.Println("H3: MUX close ", ugs.n.ID)
	}
	return ugs.s.CloseWithError(quic.ErrorCode(errorNoError), "")
}

// RoundTrip is used for forwarding HTTP connections back to the client (normal) or
// server ( reverse ).
func (ugs *QuicMUX) RoundTrip(request *http.Request) (*http.Response, error) {
	// make sure the request will be accepted
	request.URL.Scheme = "https"
	request.URL.Host = ugs.hostname
	if ugate.DebugClose {
		log.Println("H3: RT-start", ugs.n.ID, request.URL, ugs.client)
	}
	res, err := ugs.rt.RoundTrip(request)
	if ugate.DebugClose {
		log.Println("H3: RT-done", ugs.n.ID, request.URL, ugs.client, err)
		if err == nil {
			go func() {
				<-request.Context().Done()
				log.Println("H3: RT-ctx-done", ugs.n.ID, request.URL)
			}()
		}
	}
	return res, err
}

