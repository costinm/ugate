package quic

import (
	"context"

	"github.com/costinm/ugate"
	"github.com/quic-go/quic-go"

	//"io"
	"log"
	"net"
)

// QuicMUX is a mux to a specific node. May be accepted or dialed.
// Equivalent with quic/h3/client.go ( when dialing ), and Quic server when accepting.
type QuicMUX struct {
	// Remote - has an WorkloadID and Addr. Set for client connections.
	n *ugate.MeshCluster
	client   bool

	// Can be accepted or dialed - the protocol is symmetric.
	s quic.EarlyConnection

}

const errorNoError = 0x100
var Debug = false

func (ugs *QuicMUX) Close() error {
	if Debug {
		log.Println("H3: MUX close ", ugs.n.ID)
	}
	return ugs.s.CloseWithError(0, "")
}


//func (ugs *QuicMUX) DialStream(ctx context.Context, addr string, inStream util.Stream) (util.Stream, error) {
//	//if UseRawStream {
//	s, err := ugs.s.OpenStream()
//	if err != nil {
//		return nil, err
//	}
//
//	str := util.GetStream(s, s)
//	if inStream != nil && inStream.RequestHeader() != nil {
//		for k, v := range inStream.RequestHeader() {
//			str.Header()[k] = v
//		}
//	}
//
//	str.Header().Set("dest", addr)
//	err = str.SendHeader(s, str.Header())
//	if err != nil {
//		return nil, err
//	}
//
//	// RoundTripStart sending
//	if inStream != nil {
//		go func() {
//			// Equivalent with proxyToClient.
//			str.CopyBuffered(s, inStream, true)
//			// TODO: do something on err, done
//			log.Println("QUIC out - copy reader close ", str.StreamId, addr)
//
//		}()
//	}
//
//	err = str.ReadHeader(s)
//	if err != nil {
//		return nil, err
//	}
//	log.Println("QUIC out stream res", str.StreamId, addr, str.InHeader)
//
//	return str, nil
//}

// DialContext uses the current association to open a stream.
//
// Note that H3 is handled separately - this is an L4 use of QUIC, similar to SSH.
//
func (ugs *QuicMUX) DialContext(ctx context.Context, net, addr string) (net.Conn, error) {
	str, err := ugs.s.OpenStream()
	if err != nil {
		return nil, err
	}

	return &QuicNetCon{Stream: str}, nil
}

type QuicNetCon struct {
	quic.Stream
}

func (q QuicNetCon) LocalAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (q QuicNetCon) RemoteAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

//// RoundTrip is used for forwarding HTTP connections back to the client (normal) or
//// server ( reverse ).
//func (ugs *QuicMUX) RoundTrip(request *http.Request) (*http.Response, error) {
//	// make sure the request will be accepted
//	request.URL.Scheme = "https"
//	request.URL.Host = ugs.hostname
//	if Debug {
//		if ugs.n != nil {
//			log.Println("H3: RT-start", ugs.n.ID, request.URL, ugs.client)
//		} else {
//			log.Println("H3: RT-start", request.URL, ugs.client)
//		}
//	}
//	var res *http.Response
//	var err error
//	if ugs.rt != nil {
//		res, err = ugs.rt.RoundTrip(request)
//	} else {
//		// Replacement for the lack of support in QUIC for using the H3 request programmatically.
//		// This only works with h3s servers.
//		s := util.NewStream()
//		s.Request = request
//		s.In = request.Body
//
//		rs, err1 := ugs.DialStream(request.Context(), request.URL.Host, s)
//		if err1 != nil {
//			return nil, err1
//		}
//
//		res = &http.Response{
//			Body:   rs, // rs.In
//			Header: rs.Header(),
//		}
//	}
//	if Debug {
//		log.Println("H3: RT-done", request.URL, ugs.client, err)
//		if err == nil {
//			go func() {
//				<-request.Context().Done()
//				log.Println("H3: RT-ctx-done", request.URL)
//			}()
//		}
//	}
//	return res, err
//}
