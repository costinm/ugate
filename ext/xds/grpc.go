//go:generate protoc --gogofaster_out=plugins=grpc:$GOPATH/src xds.proto
package xds

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/costinm/ugate/pkg/ugatesvc"
	msgs "github.com/costinm/ugate/webpush"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type XDSService struct {
	*UnimplementedAggregatedDiscoveryServiceServer
	Mux *msgs.Mux

	// mutex used to modify structs, non-blocking code only.
	mutex sync.RWMutex

	// clients reflect active gRPC channels, for both ADS and MCP.
	// key is Connection.ConID
	clients map[string]*Connection

	connectionNumber int
}

// Connection represents a single endpoint.
// An endpoint typically has 0 or 1 connections - but during restarts and drain it may have >1.
type Connection struct {
	mu sync.RWMutex

	// PeerAddr is the address of the client envoy, from network layer
	PeerAddr string

	NodeID string

	// Time of connection, for debugging
	Connect time.Time

	// ConID is the connection identifier, used as a key in the connection table.
	// Currently based on the node name and a counter.
	ConID string

	// doneChannel will be closed when the client is closed.
	doneChannel chan int

	// Metadata key-value pairs extending the Node identifier
	Metadata map[string]string

	// Watched resources for the connection
	Watched map[string][]string

	NonceSent  map[string]string
	NonceAcked map[string]string

	// Only one can be set.
	SStream AggregatedDiscoveryService_StreamAggregatedResourcesServer
	CStream AggregatedDiscoveryService_StreamAggregatedResourcesClient

	active     bool
	resChannel chan *Response
	errChannel chan error

	// Holds node info
	firstReq *Request ``
}

// XDS and gRPC dependencies. Enabled for interop with Istio/XDS.
func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		gs := NewXDS(msgs.DefaultMux)
		grpcS := grpc.NewServer()
		// Using the native stack.
		ug.Mux.HandleFunc("/envoy.service.discovery.v3.AggregatedDiscoveryService/StreamAggregatedResources", func(writer http.ResponseWriter, request *http.Request) {
			grpcS.ServeHTTP(writer, request)
		})
		RegisterAggregatedDiscoveryServiceServer(grpcS, gs)
		//RegisterLoadReportingServiceServer(grpcS, gs)

		// TODO: register for config change, connect to upstream
		return nil
	})
}

func NewXDS(mux *msgs.Mux) *XDSService {
	g := &XDSService{Mux: mux,
		clients: map[string]*Connection{},
	}

	return g
}

var conid = 0

// Subscribe maps the the webpush subscribe request
func (s *XDSService) StreamAggregatedResources(stream AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	peerInfo, ok := peer.FromContext(stream.Context())
	peerAddr := "0.0.0.0"
	if ok {
		peerAddr = peerInfo.Addr.String()
	}

	t0 := time.Now()

	con := &Connection{
		Connect:     t0,
		PeerAddr:    peerAddr,
		SStream:     stream,
		NonceSent:   map[string]string{},
		Metadata:    map[string]string{},
		Watched:     map[string][]string{},
		NonceAcked:  map[string]string{},
		doneChannel: make(chan int, 2),
		resChannel:  make(chan *Response, 2),
		errChannel:  make(chan error, 2),
	}

	firstReq := true

	defer func() {
		if firstReq {
			return // didn't get first req, not added
		}
		close(con.resChannel)
		close(con.doneChannel)
		s.mutex.Lock()
		delete(s.clients, con.ConID)
		s.mutex.Unlock()

	}()

	// In current gRPC, a request is canceled by returning from the function.
	// If we block on Recv() - we can't cancel
	go func() {
		for {
			// Blocking. Separate go-routines may use the stream to push.
			req, err := stream.Recv()
			if err != nil {
				if status.Code(err) == codes.Canceled || err == io.EOF {
					log.Printf("ADS: %q %s terminated %v", con.PeerAddr, con.ConID, err)
					con.errChannel <- nil
					return
				}
				log.Printf("ADS: %q %s terminated with errors %v", con.PeerAddr, con.ConID, err)
				con.errChannel <- err
				return
			}

			if firstReq {
				// Node info may only be sent on the first request, save it to
				// conn.
				s.mutex.Lock()
				con.ConID = strconv.Itoa(conid)
				conid++
				s.clients[con.ConID] = con
				s.mutex.Unlock()
				con.firstReq = req
			}

			firstReq = false
			err = s.process(con, req)
			if err != nil {
				con.errChannel <- err
				return
			}
		}
	}()

	// Blocking until closed, and implement the Send channel.
	// It is not thread safe to send from different goroutines, so using
	// a chan.
	for {
		select {
		case res, _ := <-con.resChannel:
			err := stream.Send(res)
			if err != nil {
				return err
			}
		case err1, _ := <-con.errChannel:
			return err1
		}
	}

	return nil
}

func (s *XDSService) process(connection *Connection, request *Request) error {
	for _, r := range request.Resources {
		s.Mux.SendMessage(&msgs.Message{
			MessageData: msgs.MessageData{
				To:    request.TypeUrl,
				Time:  time.Now().Unix(),
				Meta:  map[string]string{},
				Topic: "",
			},
			Path:       nil,
			Data:       r,
			Connection: nil,
		})
	}
	return nil
}

func (fx *XDSService) SendAll(r *Response) {
	for _, con := range fx.clients {
		// TODO: only if watching our resource type

		r.Nonce = fmt.Sprintf("%v", time.Now())
		con.NonceSent[r.TypeUrl] = r.Nonce
		con.resChannel <- r
		// Not safe to call from 2 threads: con.SStream.Send(r)
	}
}

func (fx *XDSService) Send(con *Connection, r *Response) {
	r.Nonce = fmt.Sprintf("%v", time.Now())
	con.NonceSent[r.TypeUrl] = r.Nonce
	con.resChannel <- r
}

func Connect(addr string, clientPem string) (*grpc.ClientConn, AggregatedDiscoveryServiceClient, error) {
	opts := []grpc.DialOption{}

	// Cert file is a PEM, it is loaded into a x509.CertPool,
	// will be set as RootCAs in the tls.Config
	//creds, err := credentials.NewClientTLSFromFile("../testdata/ca.pem", "x.test.youtube.com")

	cp := x509.NewCertPool()
	cp.AppendCertsFromPEM([]byte(clientPem))

	creds := credentials.NewTLS(&tls.Config{
		// name will override ":authority" request headers,
		// also used to validate the cert ( it seems to make it to the
		// SNI header as well ?). Must match one of the cert entries
		ServerName: "x.test.youtube.com",
		// pub keys that signed the cert
		RootCAs: cp})

	if false {
		// example code: docker/libtrust/certificates.go
		cert := &x509.Certificate{
			SerialNumber: big.NewInt(0),
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			NotBefore:    time.Now().Add(-time.Hour * 24 * 7),
			NotAfter:     time.Now().Add(time.Hour * 24 * 365 * 10),
		}
		issCert := &x509.Certificate{
			SerialNumber: big.NewInt(0),
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			NotBefore:    time.Now().Add(-time.Hour * 24 * 7),
			NotAfter:     time.Now().Add(time.Hour * 24 * 365 * 10),
		}
		pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			fmt.Println(err)
		}

		certDER, err := x509.CreateCertificate(rand.Reader,
			cert, issCert, pk.Public(), pk)
		if err != nil {
			fmt.Println(err)
		}

		cp := x509.NewCertPool()
		c, err := x509.ParseCertificate(certDER)
		if err != nil {
			fmt.Println(err)
		}
		cp.AddCert(c)

		creds = credentials.NewTLS(&tls.Config{
			RootCAs: cp,
		})
		if err != nil {
			return nil, nil, err
		}
	}

	opts = append(opts, grpc.WithTransportCredentials(creds))

	// WithInsecure() == no auth, plain http. Either that or TransportCred
	// required.

	// grpc/credentials/oauth defines a number of options - it's
	// an interface called on each request, returning headers
	//
	// GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error)
	// TODO: vapid option

	// json has client_email, private_key.
	// Aud: "https://accounts.google.com/o/oauth2/token"
	// Scope: ...
	// Iss: Email
	// Signed and posted to google, which returns an oauth2 token

	// Uses 2-legged auth
	// RS256/JWT, with Iss = email, Sub = email, Aud = (google url)
	// Iat, Exp = 1h
	//json := []byte("{}")
	//jwtCreds, err := oauth.NewJWTAccessFromKey(json)

	// Other option: NewServiceAccountFromFile ( dev console service account
	// -- application default credentials )

	// golang.org/x/oauth2/google/DefaultTokenSource(ctx, scope...)
	// GOOGLE_APPLICATION_CREDENTIALS env file
	// .config/gcloud/application_default_credentials.json
	// appengine or compute engine get it from env

	// file has client_id, client_secret, refresh_token - for creds
	// and private key/key id for service accounts
	// Still goes trough Oauth2 flow.
	//gcreds, err := oauth.NewApplicationDefault(context.Background(), "test_scope")
	//gcreds.GetRequestMetadata(context.Background(), "url")

	// also NewOauthAccess - seems to allow arbitrary type/value
	// could be IID token !!!!

	//opts = append(opts, grpc.WithPerRPCCredentials(jwtCreds))

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		log.Println("Failed to dial", err)
		return nil, nil, err
	}

	return conn, NewAggregatedDiscoveryServiceClient(conn), nil
}
