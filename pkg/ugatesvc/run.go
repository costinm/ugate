package ugatesvc

import (
	"log"

	"github.com/costinm/ugate"
)

func Run(config ugate.ConfStore, g *ugate.GateCfg) (*UGate, error){
	// Start a Gate. Basic H2 and H2R services enabled.
	ug := New(config, nil, g)

	sf := []StartFunc{}
	if InitHooks != nil {
		for _, h := range InitHooks {
			s := h(ug)
			if s != nil {
				sf = append(sf, s)
			}
		}
	}

	for _, h := range sf {
		go h(ug)
	}
	ug.Start()
	log.Println("Started: ", ug.Auth.ID)
	return ug, nil
}

