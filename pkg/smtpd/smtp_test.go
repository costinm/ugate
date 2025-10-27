package smtpd

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/emersion/go-maildir"
	"github.com/emersion/go-message"
	"github.com/emersion/go-msgauth/dkim"
)

/*
const (
	// The user has resent/forwarded/bounced this message to someone else.
	FlagPassed Flag = 'P'

	// The user has replied to this message.
	FlagReplied Flag = 'R'

	// The user has viewed this message, though perhaps he didn't read all the
	// way through it.
	FlagSeen Flag = 'S'

	// The user has moved this message to the trash; the trash will be emptied
	// by a later user action.
	FlagTrashed Flag = 'T'

	// The user considers this message a draft; toggled at user discretion.
	FlagDraft Flag = 'D'

	// User-defined flag; toggled at user discretion.
	FlagFlagged Flag = 'F'
)
 */


func TestSMTP(t *testing.T) {
	msg0 := `Hello: world
From: costin@example.com

Message`

	t.Run("dir", func(t *testing.T) {
		d := maildir.Dir("testdata")
		err := d.Init()
		if err != nil {
			t.Fatal(err)
		}


		// Flags are part of the filename, can be changed with a rename
		// Also easy to filter without open
		m, iwc, err := d.Create([]maildir.Flag{maildir.FlagDraft})
		defer iwc.Close()
		io.WriteString(iwc, msg0)
		iwc.Close()

		irw, err := m.Open()
		ba, err := io.ReadAll(irw)
		log.Println(string(ba))

		d.Walk(func(msg *maildir.Message) error {
			log.Println(msg.Key())
			r, _ := msg.Open()
			e, _ := message.Read(r)
			log.Println(e)
			return nil
		})

	})


	t.Run("dkim", func(t *testing.T) {

	r := strings.NewReader(msg0)

	// only rsa and ed25519 supported by this library (easy to patch)

	//kp, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	pubk, kp, _ := ed25519.GenerateKey(rand.Reader)

	options := &dkim.SignOptions{
		Domain:   "example.org",
		// Domain may have many keys. This could be used as a user id
		// or for subdomain
		Selector: "k0",
		Signer:   kp,
		//QueryMethods: "dns/txt",
	}

	var b bytes.Buffer
	if err := dkim.Sign(&b, r, options); err != nil {t.Fatal(err)
	}

	pubk64 := base64.StdEncoding.EncodeToString(pubk)
	res, err := dkim.VerifyWithOptions(bytes.NewReader(b.Bytes()), &dkim.VerifyOptions{
		LookupTXT: func(domain string) ([]string, error) {
			log.Println("TXT lookup: ", domain)
			return []string{"v=DKIM1; k=ed25519; p=" + pubk64}, nil
		},
	})
	log.Println(res[0], err)
	log.Println(string(b.Bytes()))
	})

}
