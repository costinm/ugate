package xds

import (
	"fmt"
	"log"
	"time"
)

// Fetch waits for the basic responses - CDS, EDS, RDS, LDS.
// Watch() must be called before.
func (con *ADSC) Fetch() (*Responses, error) {
	logf := log.Printf
	watchAll := map[string]struct{}{"cds": {}, "eds": {}, "rds": {}, "lds": {}}

	exit := false
	for !exit {
		select {
		case u := <-con.Updates:
			if u == "close" {
				// Close triggered. This may mean Pilot is just disconnecting, scaling, etc
				// Try the whole loop again
				logf("Closing XDS")
				exit = true
			}
			delete(watchAll, u)
			if len(watchAll) == 0 {
				logf("Done: XDS")
				exit = true
			}
		case <-con.Config.Context.Done():
			return nil, fmt.Errorf("context closed")
		}
	}

	return &con.Responses, nil
}

// Connect dials and stays connected, retrying in case of errors.
func (a *ADSC) Connect(pilotAddress string) {
	config := a.Config
	attempts := 0
	logf := log.Printf
	for {
		t0 := time.Now()
		logf("Connecting: %v", config.IP)
		con, err := Dial(pilotAddress, config)
		if err != nil {
			logf("Error in ADS connection: %v", err)
			attempts++
			select {
			case <-config.Context.Done():
				logf("Context closed, exiting stream")
				con.Close()
				return
			case <-time.After(time.Second * time.Duration(attempts)):
				logf("Starting retry %v", attempts)
			}
			continue
		}

		logf("Connected: %v in %v", config.IP, time.Since(t0))

		con.Watch()

		update := false
		exit := false
		for !exit {
			select {
			case u := <-con.Updates:
				if u == "close" {
					// Close triggered. This may mean Pilot is just disconnecting, scaling, etc
					// Try the whole loop again
					logf("Closing: %v", config.IP)
					exit = true
				} else if !update {
					update = true
					logf("Got Initial Update: %v for %v in %v", config.IP, u, time.Since(t0))
				}
			case <-config.Context.Done():
				// We are really done now. Shut everything down and stop
				logf("Context closed, exiting stream")
				con.Close()
				return
			}
		}
		logf("Disconnected: %v", config.IP)
		time.Sleep(time.Millisecond * 500)
	}
}
