package xds

//
//import (
//	"fmt"
//	"io"
//	"log"
//	"net/http"
//	"strconv"
//	"sync"
//	"time"
//
//	msgs "github.com/costinm/ugate/webpush"
//	"google.golang.org/grpc/status"
//)
//
//type XDSService struct {
//	Mux *msgs.Mux
//
//	// mutex used to modify structs, non-blocking code only.
//	mutex sync.RWMutex
//
//	// clients reflect active gRPC channels
//	// key is Connection.ConID
//	clients map[string]*Connection
//
//	connectionNumber int
//}
//
//// Connection represents a single endpoint.
//// An endpoint typically has 0 or 1 connections - but during restarts and drain it may have >1.
//type Connection struct {
//	mu sync.RWMutex
//
//	// PeerAddr is the address of the client envoy, from network layer
//	PeerAddr string
//
//	NodeID string
//
//	// Time of connection, for debugging
//	Connect time.Time
//
//	// ConID is the connection identifier, used as a key in the connection table.
//	// Currently based on the node name and a counter.
//	ConID string
//
//	// doneChannel will be closed when the client is closed.
//	doneChannel chan int
//
//	// Metadata key-value pairs extending the Node identifier
//	Metadata map[string]string
//
//	// Watched resources for the connection
//	Watched map[string][]string
//
//	NonceSent  map[string]string
//	NonceAcked map[string]string
//
//	active     bool
//	resChannel chan *Response
//	errChannel chan error
//
//	// Holds node info
//	firstReq *Request ``
//}
//
////// XDS and gRPC dependencies. Enabled for interop with Istio/XDS.
////func init() {
////	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
////		gs := NewXDS(msgs.DefaultMux)
////
////		// Using the native stack.
////		ug.Mux.HandleFunc("/envoy.service.discovery.v3.AggregatedDiscoveryService/StreamAggregatedResources", func(writer http.ResponseWriter, request *http.Request) {
////			gs.ServeHTTP(writer, request)
////		})
////		//RegisterLoadReportingServiceServer(grpcS, gs)
////
////		// TODO: register for config change, connect to upstream
////		return nil
////	})
////}
//
//func NewXDS(mux *msgs.Mux) *XDSService {
//	g := &XDSService{Mux: mux,
//		clients: map[string]*Connection{},
//	}
//
//	return g
//}
//
//var conid = 0
//
//// Subscribe maps the the webpush subscribe request
//func (s *XDSService) StreamAggregatedResources(stream AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
//	peerInfo, ok := peer.FromContext(stream.Context())
//	peerAddr := "0.0.0.0"
//	if ok {
//		peerAddr = peerInfo.Addr.String()
//	}
//
//	t0 := time.Now()
//
//	con := &Connection{
//		Connect:     t0,
//		PeerAddr:    peerAddr,
//		SStream:     stream,
//		NonceSent:   map[string]string{},
//		Metadata:    map[string]string{},
//		Watched:     map[string][]string{},
//		NonceAcked:  map[string]string{},
//		doneChannel: make(chan int, 2),
//		resChannel:  make(chan *Response, 2),
//		errChannel:  make(chan error, 2),
//	}
//
//	firstReq := true
//
//	defer func() {
//		if firstReq {
//			return // didn't get first req, not added
//		}
//		close(con.resChannel)
//		close(con.doneChannel)
//		s.mutex.Lock()
//		delete(s.clients, con.ConID)
//		s.mutex.Unlock()
//
//	}()
//
//	// In current gRPC, a request is canceled by returning from the function.
//	// If we block on Recv() - we can't cancel
//	go func() {
//		for {
//			// Blocking. Separate go-routines may use the stream to push.
//			req, err := stream.Recv()
//			if err != nil {
//				if status.Code(err) == codes.Canceled || err == io.EOF {
//					log.Printf("ADS: %q %s terminated %v", con.PeerAddr, con.ConID, err)
//					con.errChannel <- nil
//					return
//				}
//				log.Printf("ADS: %q %s terminated with errors %v", con.PeerAddr, con.ConID, err)
//				con.errChannel <- err
//				return
//			}
//
//			if firstReq {
//				// Node info may only be sent on the first request, save it to
//				// conn.
//				s.mutex.Lock()
//				con.ConID = strconv.Itoa(conid)
//				conid++
//				s.clients[con.ConID] = con
//				s.mutex.Unlock()
//				con.firstReq = req
//			}
//
//			firstReq = false
//			err = s.process(con, req)
//			if err != nil {
//				con.errChannel <- err
//				return
//			}
//		}
//	}()
//
//	// Blocking until closed, and implement the Send channel.
//	// It is not thread safe to send from different goroutines, so using
//	// a chan.
//	for {
//		select {
//		case res, _ := <-con.resChannel:
//			err := stream.Send(res)
//			if err != nil {
//				return err
//			}
//		case err1, _ := <-con.errChannel:
//			return err1
//		}
//	}
//
//	return nil
//}
//
//func (s *XDSService) process(connection *Connection, request *Request) error {
//	for _, r := range request.Resources {
//		s.Mux.SendMessage(&msgs.Message{
//			MessageData: msgs.MessageData{
//				To:    request.TypeUrl,
//				Time:  time.Now().Unix(),
//				Meta:  map[string]string{},
//				Topic: "",
//			},
//			Path:       nil,
//			Data:       r,
//			Connection: nil,
//		})
//	}
//	return nil
//}
//
//func (fx *XDSService) SendAll(r *Response) {
//	for _, con := range fx.clients {
//		// TODO: only if watching our resource type
//
//		r.Nonce = fmt.Sprintf("%v", time.Now())
//		con.NonceSent[r.TypeUrl] = r.Nonce
//		con.resChannel <- r
//		// Not safe to call from 2 threads: con.SStream.Send(r)
//	}
//}
//
//func (fx *XDSService) Send(con *Connection, r *Response) {
//	r.Nonce = fmt.Sprintf("%v", time.Now())
//	con.NonceSent[r.TypeUrl] = r.Nonce
//	con.resChannel <- r
//}
//
//func (s *XDSService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
//
//}
