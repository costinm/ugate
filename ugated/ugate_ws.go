package ugated

import (
	"net/http"

	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/gorilla/websocket"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		w := &ws{ug: ug}

		ug.Mux.Handle("/ws/", w)

		return nil
	})
}

type ws struct {
	ug *ugatesvc.UGate
}

// Integrate with Websocket as a server, equvalent with a connect.
// This can also be used for SSH over WS instead of HTTPS over WS.
func (ws *ws) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u := websocket.Upgrader{}
	u.Upgrade(w, r, http.Header{})

}

// "golang.org/x/net/websocket" - not supported. RoundTripStart only text frames.
//
//func Server() http.Handler {
//	wsmsg := &ws.Server{
//		Config:    ws.Config{},
//		Handshake: nil,
//		Handler: func(conn *ws.Stream) {
//			//h2ctx := auth.AuthContext(conn.Request().Context())
//			//websocketStream(conn)
//		},
//	}
//	return wsmsg
//	//mux.Handle("/ws", wsmsg)
//}
//
//func Client(dest string) (*ws.Stream, error) {
//	wsc, err := ws.NewConfig(dest, dest)
//
//	//wsc.Header.Add("Authorization", a.VAPIDToken(dest))
//
//	wsc.TlsConfig = &tls.Config{
//		InsecureSkipVerify: true,
//	}
//
//	ws, err := ws.DialConfig(wsc)
//	if err != nil {
//		return nil, err
//	}
//
//	return ws, err
//}
