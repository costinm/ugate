package nio2

import (
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

var ProxyCnt atomic.Int32

// Proxy forwards from nc to in/w.
// outConn is typically the result of DialContext - egressing.
// dest is used for logging and tracking.
func Proxy(outConn net.Conn, in io.Reader, w io.Writer, dest string) error {
	t1 := time.Now()

	ch := make(chan int)
	ch2 := make(chan int)

	s1 := &ReaderCopier{
		Out: outConn,
		In:  in,
	}
	s2 := &ReaderCopier{
		Out: w,
		In:  outConn,
	}
	ids := ""
	if dest != "" {
		id := ProxyCnt.Add(1)
		ids = strconv.Itoa(int(id))
		s1.ID = dest + "-o-" + ids
		s2.ID = dest + "-i-" + ids
	}
	go s1.Copy(ch, true)

	go s2.Copy(ch2, true)

	for i := 0; i < 2; i++ {
		select {
		case <-ch:
			if s1.Err != nil {
				s2.Close()
				break
			}
			if Debug {
				log.Println("Proxy in done", ids, s1.Err, s1.InError, s1.Written)
			}
		case <-ch2:
			if s2.Err != nil {
				s1.Close()
				break
			}
			if Debug {
				log.Println("Proxy out done", ids, s2.Err, s2.InError, s2.Written)
			}
		}
	}

	err := proxyError(s1.Err, s2.Err, s1.InError, s2.InError)

	if dest != "" {
		log.Println("proxy-copy-done", ids,
			dest,
			//"conTime", t1.Sub(t0),
			"dur", time.Since(t1),
			"maxRead", s1.MaxRead, s2.MaxRead,
			"readCnt", s1.ReadCnt, s2.ReadCnt,
			"avgCnt", int(s1.Written)/(s1.ReadCnt+1), int(s2.Written)/(s2.ReadCnt+1),
			"in", s1.Written,
			"out", s2.Written,
			"err", err)
	}

	outConn.Close()
	if c, ok := in.(io.Closer); ok {
		c.Close()
	}
	if c, ok := w.(io.Closer); ok {
		c.Close()
	}

	return err
}

func proxyError(errout error, errorin error, outInErr bool, inInerr bool) error {
	if errout == nil && errorin == nil {
		return nil
	}
	if errout == nil {
		return errorin
	}
	if errorin == nil {
		return errout
	}

	return errors.New("IN+OUT " + errorin.Error() + " " + errout.Error())
}
