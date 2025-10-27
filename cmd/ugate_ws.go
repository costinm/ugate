package cmd

import (
	"net/http"

	"github.com/costinm/ugate/appinit"
	"github.com/gorilla/websocket"
)

func RegisterWS() {
	appinit.RegisterT("WS", &WS{})
}

type WS struct {
}

// Integrate with Websocket as a server, equvalent with a connect.
// This can also be used for SSH over WS instead of HTTPS over WS.
func (ws *WS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u := websocket.Upgrader{}
	nc, err := u.Upgrade(w, r, http.Header{})
	if err != nil {
		w.WriteHeader(500)
		return
	}
	// nc is a raw connection - we still have access to headers.
	// For now - just start a SSH connection over WS
	nc.Close()
}

// "golang.org/x/net/websocket" - not supported. RoundTripStart only text frames.
//
//func Server() http.Handler {
//	wsmsg := &WS.Server{
//		Config:    WS.Config{},
//		Handshake: nil,
//		Handler: func(conn *WS.Stream) {
//			//h2ctx := auth.AuthContext(conn.Request().Context())
//			//websocketStream(conn)
//		},
//	}
//	return wsmsg
//	//mux.Handle("/WS", wsmsg)
//}
//
//func Client(dest string) (*WS.Stream, error) {
//	wsc, err := WS.NewConfig(dest, dest)
//
//	//wsc.Header.Add("Authorization", a.VAPIDToken(dest))
//
//	wsc.TlsConfig = &tls.Config{
//		InsecureSkipVerify: true,
//	}
//
//	WS, err := WS.DialConfig(wsc)
//	if err != nil {
//		return nil, err
//	}
//
//	return WS, err
//}
