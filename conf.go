package ugate

import (
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"encoding/json"
)

// Helpers for very simple configuration and key loading.
// Will use json files for config.

type ConfStore interface {
	// Get a config blob by name
	Get(name string) ([]byte, error)

	// Save a config blob
	Set(conf string, data []byte) error

	// List the configs starting with a prefix, of a given type
	List(name string, tp string) ([]string, error)
}

func ConfStr(cs ConfStore, name, def string) string {
	if cs == nil {
		return def
	}
	b, _ := cs.Get(name)
	if b == nil {
		return def
	}
	return string(b)
}

func ConfInt(cs ConfStore, name string, def int) int {
	if cs == nil {
		return def
	}
	b, _ := cs.Get(name)
	if b == nil {
		return def
	}
	v, err := strconv.Atoi(string(b))
	if err != nil {
		return def
	}
	return v
}

// Simple file-based config and secret store.
//
// Implements a ConfStore interface with List/Get/Set interface.
// TODO: Watch interface - using messages/pubsub !!!
//
// TODO: switch to yaml, support K8S style
//
type Conf struct {
	// Base directory. If not set, no config will be saved and read will fail.
	base []string
	// Conf is configured from Android side with the config (settings)
	// ssid, pass, vpn_ext
	Conf map[string]string `json:"Conf,omitempty"`
}

// Returns a config store.
// Implements a basic auth.ConfStore interface
func NewConf(base ...string) *Conf {
	// TODO: https for remote - possibly using local creds and K8S style or XDS
	return &Conf{base: base, Conf: map[string]string{}}
}

func (c *Conf) List(name string, tp string) ([]string, error) {
	return nil, nil
}

func Get(h2 *Conf, name string, to interface{}) error {
	raw, err := h2.Get(name)
	if err != nil {
		log.Println("name:", err)
		raw = []byte("{}")
		//return nil
	}
	if err := json.Unmarshal(raw, to); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

// Secrets - pem, acl
// From config dir, fallback to .sshterraform, .lego and /etc/certs
//
// "name" may be a hostname
func (c *Conf) Get(name string) ([]byte, error) {
	envName := strings.ReplaceAll(name, ".", "_")
	envName = strings.ReplaceAll(envName, "/", "_")
	envd := os.Getenv(envName)
	if envd != "" {
		return []byte(envd), nil
	}

	for _, b := range c.base {
		l := b + name

		if _, err := os.Stat(l); err == nil { // || !os.IsNotExist(err)
			res, err := ioutil.ReadFile(l)
			if err == nil {
				return res, nil
			}
		}
		if _, err := os.Stat(l + ".json"); err == nil { // || !os.IsNotExist(err)
			res, err := ioutil.ReadFile(l + ".json")
			if err == nil {
				return res, nil
			}
		}
	}

	// name may be a hostname - use it to load ACME certificate for the host.

	return nil, nil
}

func (c *Conf) Set(conf string, data []byte) error {
	if c == nil || c.base == nil || len(c.base) == 0 {
		return nil
	}
	err := ioutil.WriteFile(c.base[0]+conf, data, 0700)
	if err != nil {
		log.Println("Error saving ", err, c.base, conf)
	}
	return err
}
