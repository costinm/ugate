package echo

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/costinm/meshauth"
	"github.com/costinm/meshauth/pkg/certs"
	"github.com/costinm/ssh-mesh/pkg/h2"
	"github.com/costinm/ugate/nio2"
)

// Control handler, also used for testing
type EchoHandler struct {
	UGate *meshauth.Mesh

	Debug       bool
	ServerFirst bool
	WaitFirst   time.Duration

	Received int
}

var DebugEcho = false


func (eh *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if eh.Debug {
		log.Println("ECHOH ", r)
	}
	w.WriteHeader(200)

	// H2 requests require write to be flushed - buffering happens !

	if r.URL.Port() == "/raw" {
		// Similar with echo TCP - but can't close
		eh.handleStreams(r.Body, w)
		return
	}

	w.(http.Flusher).Flush()
	//// Wrap w.Body into Stream which does this automatically
	str := h2.NewStreamServerRequest(r, w)
	//
	eh.handle(str, false)
}

func (eh *EchoHandler) EchoHTTP(writer http.ResponseWriter, request *http.Request) {
	host := request.Host
	fmt.Fprintf(writer, "host: %s, req: %s, headers: %v", host, request.RequestURI, request.Header)
	b := make([]byte, 1024)
	for {
		n, err := request.Body.Read(b)
		if err != nil {
			fmt.Fprintf(writer, "Err %v", err)
			return
		}
		log.Println(b[0:n])
		writer.Write(b[0:n])
	}
}


func (eh *EchoHandler) Wait(writer http.ResponseWriter, request *http.Request) {
	host := request.Host
	fmt.Fprintf(writer, "host: %s, req: %s, headers: %v", host, request.RequestURI, request.Header)
	time.Sleep(60 * time.Second)

}

func (eh *EchoHandler) RegisterMux(mux *http.ServeMux)  {
		mux.HandleFunc("/echo/", eh.EchoHTTP)
		mux.HandleFunc("/wait/", eh.Wait)
}


// StreamInfo tracks information about one stream.
type StreamInfo struct {
	LocalAddr  net.Addr
	RemoteAddr net.Addr

	Meta http.Header

	RemoteID string

	ALPN string

	Dest string

	Type string
}

func GetStreamInfo(str net.Conn) *StreamInfo {
	si := &StreamInfo{
		LocalAddr:  str.LocalAddr(),
		RemoteAddr: str.RemoteAddr(),
	}
	if s, ok := str.(nio2.StreamMeta); ok {
		si.Meta = s.RequestHeader()
		// TODO: extract identity - including UDS
		if tc := s.TLSConnectionState(); tc != nil {
			si.ALPN = tc.NegotiatedProtocol
		}
	}

	return si
}


func (e *EchoHandler) handleStreams(in io.Reader, out io.Writer) {
	d := make([]byte, 2048)
	b := &bytes.Buffer{}

	b.WriteString("Hello world\n")

	time.Sleep(e.WaitFirst)

	if e.ServerFirst {
		n, err := out.Write(b.Bytes())
		if e.Debug {
			log.Println("ServerFirst write()", n, err)
		}
	}
	writeClosed := false
	for {
		n, err := in.Read(d)
		e.Received += n
		if e.Debug {
			log.Println("Echo read()", n, err)
		}
		if err != nil {
			if e.Debug {
				log.Println("ECHO DONE", err)
			}
			if err == io.EOF && e.ServerFirst {
				binary.BigEndian.PutUint32(d, uint32(n))
				out.Write(d[0:4])
				if cw, ok := out.(nio2.CloseWriter); ok {
					cw.CloseWrite()
				}
			} else {
				if c, ok := in.(io.Closer); ok {
					c.Close()
				}
				if c, ok := out.(io.Closer); ok {
					c.Close()
				}
			}
			return
		}

		// Client requests server graceful close
		if d[0] == 0 {
			if wc, ok := out.(nio2.CloseWriter); ok {
				wc.CloseWrite()
				writeClosed = true
				// Continue to read ! The test can check the read byte counts
			}
		}

		if !writeClosed {
			// TODO: add delay (based on req)
			out.Write(d[0:n])
			if e.Debug {
				log.Println("ECHO write")
			}
		}
		if f, ok := out.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// RemoteID returns the node WorkloadID based on authentication.
func RemoteID(s nio2.Stream) string {
	tls := s.TLSConnectionState()
	if tls != nil {
		if len(tls.PeerCertificates) == 0 {
			return ""
		}
		pk, err := certs.VerifyChain(tls.PeerCertificates)
		if err != nil {
			return ""
		}

		return certs.PublicKeyBase32SHA(pk)
	}
	return ""
}

func (e *EchoHandler) handle(str net.Conn, serverFirst bool) error {
	d := make([]byte, 2048)

	si := GetStreamInfo(str)
	//si.RemoteID = RemoteID(str)

	b1, _ := json.Marshal(si)
	b := &bytes.Buffer{}

	b.Write(b1)
	b.Write([]byte{'\n'})

	if serverFirst {
		str.Write(b.Bytes())
	}
	//ac.SetDeadline(time.Now().StartListener(5 * time.Second))
	n, err := str.Read(d)
	if err != nil {
		return err
	}
	//if DebugEcho {
	//	log.Println("ECHO rcv", n, "strid", str.State().StreamId)
	//}
	if !serverFirst {
		str.Write(b.Bytes())
	}
	str.Write(d[0:n])

	io.Copy(str, str)
	//if DebugClose {
	//	log.Println("ECHO DONE", str.StreamId)
	//}
	return nil
}
func (eh *EchoHandler) String() string {
	return "Echo"
}
func (eh *EchoHandler) HandleConn(conn net.Conn) error {
	s := conn.(nio2.Stream)
	return eh.Handle(s)
}

func (eh *EchoHandler) Handle(ac nio2.Stream) error {
	if DebugEcho {
		log.Println("ECHOS ", ac)
	}
	defer ac.Close()
	return eh.handle(ac, false)
}


func (e *EchoHandler) ServeTCP(conn net.Conn) error {
	if e.Debug {
		log.Println("Echo ", e.ServerFirst, conn.RemoteAddr())
	}
	e.handleStreams(conn, conn)

	return nil
}

