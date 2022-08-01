package h2r

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/costinm/ugate"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// WIP: Low-level H2 - to allow reverse streams and more efficient proxy.
// Based on x/net/http2/transport.go - but with push support
// See h2i, spdystream, etc
// Uses 'spdy' ALPN on TLS, can be used over existing streams to multiplex.
//
// For better stability:
// - use the golang H2 implementation by default, over TLS or websocket (until this file is very stable)
// - client create a POST stream for accepted connections, over H2
// -

// NewStream opens a H2 stream. No H2 header (empty) - this matches QUIC streams
func (c *H2RMux) NewStream(ctx context.Context, req *http.Request) (*H2Stream, error) {
	c.m.Lock()
	id := c.nextStreamID
	c.nextStreamID += 2
	c.m.Unlock()

	s := c.stream(id)

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	path := req.RequestURI
	if path == "" {
		path = "/"
	}

	// TODO: support for 'PUSH' frames for webpush.
	s.henc.WriteField(hpack.HeaderField{Name: ":authority", Value: host})
	s.henc.WriteField(hpack.HeaderField{Name: ":method", Value: req.Method})
	s.henc.WriteField(hpack.HeaderField{Name: ":path", Value: path})
	s.henc.WriteField(hpack.HeaderField{Name: ":scheme", Value: "https"})

	s.hbuf.Reset()
	for k, vv := range req.Header {
		lowKey := strings.ToLower(k)
		if lowKey == "host" {
			continue
		}
		for _, v := range vv {
			s.henc.WriteField(hpack.HeaderField{Name: lowKey, Value: v})
		}
	}
	err := c.framer.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      id,
		EndHeaders:    true,
		BlockFragment: s.hbuf.Bytes(),
	})

	return s, err
}

func (h2c *H2RMux) serve() error {
	// TODO: Settings handshake

	var str *H2Stream
	for {
		f, err := h2c.framer.ReadFrame()
		if err != nil {
			return err
		}
		log.Println("H2 F: ", f.Header().Type)

		switch f := f.(type) {
		case *http2.SettingsFrame:
			// Sender acknoweldged the SETTINGS frame. No need to write
			// SETTINGS again.
			if f.IsAck() {
				break
			}
			if err := h2c.framer.WriteSettingsAck(); err != nil {
				return nil
			}

		case *http2.PingFrame:

		case *http2.PushPromiseFrame:

		case *http2.GoAwayFrame: // not used for server, usually servers send GO_AWAY.

		case *http2.WindowUpdateFrame:
			str = h2c.stream(f.StreamID)

		case *http2.DataFrame:
			str = h2c.stream(f.StreamID)
			// TODO: flow control, see h2
			if f.Length > 0 {
				h2c.framer.WriteWindowUpdate(f.StreamID, f.Length)
				h2c.framer.WriteWindowUpdate(0, f.Length)
			}
			if str == nil {
				// TODO: RST
				continue
			}
			str.dataChan <- f

		case *http2.RSTStreamFrame:
			str = h2c.stream(f.StreamID)

		case *http2.ContinuationFrame:
			str = h2c.stream(f.StreamID)
			if _, err := str.hdec.Write(f.HeaderBlockFragment()); err != nil {
				return nil
			}
			if f.FrameHeader.Flags&http2.FlagHeadersEndHeaders != 0 {
				h2c.handleStream(str)
			}

		case *http2.HeadersFrame:
			str := h2c.stream(f.StreamID)
			if str == nil {
				str = h2c.addStream(f.StreamID, f)
			}
			if _, err := str.hdec.Write(f.HeaderBlockFragment()); err != nil {
				return nil
			}
			if f.FrameHeader.Flags&http2.FlagHeadersEndHeaders != 0 {
				h2c.handleStream(str)
			}
		}
	}
}

func (h2s *H2RMux) closeStream(id uint32) {
	h2s.m.Lock()
	delete(h2s.streams, id)
	h2s.m.Unlock()
}

func (h2s *H2RMux) addStream(id uint32, f *http2.HeadersFrame) *H2Stream {
	h2s.m.Lock()
	// TODO: reuse
	bb := &bytes.Buffer{}
	ss := &H2Stream{
		s:    h2s,
		hbuf: bb,
		henc: hpack.NewEncoder(bb),
		hdec: hpack.NewDecoder(uint32(4<<10), func(hf hpack.HeaderField) {
			log.Println("Header: ", hf.Name, hf.Value)
		}),
	}
	h2s.streams[id] = ss
	h2s.m.Unlock()

	return ss

}

func (h2s *H2RMux) stream(id uint32) *H2Stream {
	h2s.m.RLock()
	if ss, f := h2s.streams[id]; f {
		h2s.m.RUnlock()
		return ss
	}
	h2s.m.RUnlock()
	return nil
}

// H2Stream is a multiplexed stream.
type H2Stream struct {
	meta *ugate.Stream

	id *uint32
	s  *H2RMux

	hbuf *bytes.Buffer // HPACK encoder writes into this
	hdec *hpack.Decoder
	henc *hpack.Encoder

	dataChan chan *http2.DataFrame
	// Callback style - if set dataChan and Read will not be used. Not blocking
	DataHandlerNB func(*http2.DataFrame) error

	readClosed bool
	unread     []byte
}

func (str *H2Stream) Close() error {
	return nil
}

func (str *H2Stream) Write(p []byte) (n int, err error) {
	panic("implement me")
}

func (str *H2Stream) Read(p []byte) (n int, err error) {
	if len(str.unread) > 0 {

	}
	f := <-str.dataChan
	if f.StreamEnded() {

	}

	panic("implement me")
}
