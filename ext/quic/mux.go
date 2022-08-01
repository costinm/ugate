package quic

import (
	"context"
	//"io"
	"log"
	"net"
	"net/http"

	"github.com/costinm/ugate"
	"github.com/lucas-clemente/quic-go"
)

// QuicMUX is a mux to a specific node. May be accepted or dialed.
// Equivalent with quic/h3/client.go ( when dialing ), and Quic server when accepting.
type QuicMUX struct {
	// Remote - has an ID and Addr
	n *ugate.DMNode

	// Can be accepted or dialed - the protocol is symmetric.
	s quic.EarlySession

	// quic/http3 client - currently used for sending the headers.
	// It is not the most efficient - for TCP we can avoid the extra DATA framing
	// and use simpler headers.
	rt http.RoundTripper

	// required by client - there is an extra check on the URL
	hostname string
	client   bool
}

var UseRawStream = true

const errorNoError = 0x100

func (ugs *QuicMUX) Close() error {
	if ugate.DebugClose {
		log.Println("H3: MUX close ", ugs.n.ID)
	}
	return ugs.s.CloseWithError(quic.ErrorCode(errorNoError), "")
}

func (q *Quic) handleRaw(qs quic.Stream) {
	str := ugate.NewStream()
	str.In = qs
	str.Out = qs
	err := str.ReadHeader(qs)
	if err != nil {
		log.Println("Receive error ", err)
		return
	}
	str.Dest = str.InHeader.Get("dest")
	log.Println("QUIC stream IN", str.StreamId, str.Dest)

	str.PostDialHandler = func(conn net.Conn, err error) {
		str.Header().Set("status", "200")
		str.SendHeader(qs, str.Header())
		log.Println("QUIC stream IN rcv", str.StreamId, str.InHeader)
	}

	q.UG.HandleVirtualIN(str)

}

func (ugs *QuicMUX) DialStream(ctx context.Context, addr string, inStream *ugate.Stream) (*ugate.Stream, error) {
	//if UseRawStream {
	s, err := ugs.s.OpenStream()
	if err != nil {
		return nil, err
	}
	str := ugate.NewStream()
	// if strarts with /dm/...
	str.In = s
	str.Out = s
	if inStream != nil && inStream.InHeader != nil {
		for k, v := range inStream.InHeader {
			str.Header()[k] = v
		}
	}
	str.Header().Set("dest", addr)
	err = str.SendHeader(s, str.Header())
	if err != nil {
		return nil, err
	}
	// Start sending
	if inStream != nil {
		go func() {
			// Equivalent with proxyToClient.
			str.CopyBuffered(s, inStream, true)
			// TODO: do something on err, done
			log.Println("QUIC out - copy reader close ", str.StreamId, addr)

		}()
	}

	err = str.ReadHeader(s)
	if err != nil {
		return nil, err
	}
	log.Println("QUIC out stream res", str.StreamId, addr, str.InHeader)

	return str, nil
	//} else {
	//var in io.Reader
	//var out io.WriteCloser
	//if inStream == nil {
	//	in, out = io.Pipe()
	//} else {
	//	in = inStream
	//}
	//
	//// Regular TCP stream, upgraded to H2.
	//// This is a simple tunnel, so use the right URL
	//r1, err := http.NewRequestWithContext(ctx, "POST",
	//	"https://"+ugs.hostname+"/dm/"+addr, in)
	//
	//// RoundTrip Transport guarantees this is set
	//if r1.Header == nil {
	//	r1.Header = make(http.Header)
	//}
	//
	//// RT client - forward the request.
	//res, err := ugs.RoundTrip(r1)
	//if err != nil {
	//	log.Println("H2R error", addr, err)
	//	return nil, err
	//}
	//
	//rs := ugate.NewStreamRequestOut(r1, out, res, nil)
	//if ugate.DebugClose {
	//	log.Println("TUN: ", addr, r1.URL)
	//}
	//return rs, nil

	//}
}

// RoundTrip is used for forwarding HTTP connections back to the client (normal) or
// server ( reverse ).
func (ugs *QuicMUX) RoundTrip(request *http.Request) (*http.Response, error) {
	// make sure the request will be accepted
	request.URL.Scheme = "https"
	request.URL.Host = ugs.hostname
	if ugate.DebugClose {
		if ugs.n != nil {
			log.Println("H3: RT-start", ugs.n.ID, request.URL, ugs.client)
		} else {
			log.Println("H3: RT-start", request.URL, ugs.client)
		}
	}
	var res *http.Response
	var err error
	if ugs.rt != nil {
		res, err = ugs.rt.RoundTrip(request)
	} else {
		// Replacement for the lack of support in QUIC for using the H3 request programmatically.
		// This only works with h3s servers.
		s := ugate.NewStream()
		s.Request = request
		s.In = request.Body
		rs, err1 := ugs.DialStream(request.Context(), request.URL.Host, s)
		if err1 != nil {
			return nil, err1
		}
		res = &http.Response{
			Body:   rs.In,
			Header: rs.InHeader,
		}
	}
	if ugate.DebugClose {
		log.Println("H3: RT-done", request.URL, ugs.client, err)
		if err == nil {
			go func() {
				<-request.Context().Done()
				log.Println("H3: RT-ctx-done", request.URL)
			}()
		}
	}
	return res, err
}
