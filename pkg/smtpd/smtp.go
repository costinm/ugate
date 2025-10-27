package smtpd

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/emersion/go-maildir"
	"github.com/emersion/go-smtp"
)

/*
Idea: a 'receive only' SMTP server, using unique, opaque email addresses like
atproto PLC (i.e. hashes of public keys and data).

A user will generate different email addresses for different sites or people.
IF someone shares the email - it can be identified, and different sites can't
track the user by email. Similar to WebAuthn model. OAuth2 also provides means to
return a UID (not the real ID), but rarely used.

The server will keep a database of supported IDs, reject anything else.

The messages will be passed to other modules, which may dynamically process them
like a http server or send them using other protocols (chat, pubsub, ATproto, webpush). May also be saved and accessed using IMAP/POP3, as a separate module.

Option: if the message is signed (DKIM) and sender is known, accept the message.

The From: field can be replaced with a label associated with the ID.

Strict and low limits on message size (not accepting pictures or large docs,
unlike a normal SMTP server).

Signing

- DKIM - using keys from DNS (but can be any key), includes list of signed headers.
	- DKIM-Signature header
	- jellevander/dkim -
	- https://github.com/emersion/go-msgauth
- ARC - for trusted middle boxes, uses DKIM - but still require trust.

Gateways:
- add a Authentication-Result field
- new signature, including AR
- adds ArcSeal

A list server is original use case, needs to change From - and verifies and resigns.

A better solution may be to pass original - and a 'patch' signed by the list server and applied by receiver if it trusts.


 */

// The Server implements SMTP server methods.
type Server struct {
	s *smtp.Server

	NetListener net.Listener
	Domain      string
	Addr        string
	dir         maildir.Dir
}

// NewSession is called after client greeting (EHLO, HELO).
func (bkd *Server) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Conn{
		Conn: c,
		BE: bkd,
	}, nil
}

// A Conn is returned after successful login.
// Can handle multiple messages.
type Conn struct {
	Conn *smtp.Conn
	BE   *Server

	// Return path - may be different from the From in the message.
	// Not relevant - we'll not return anything, but we can repurpose it for
	// a verified email.
	From string

	// Options in the MAIL command
	MailOpts *smtp.MailOptions

	// Original recipient, etc
	RcptOpts *smtp.RcptOptions

	To []string
}

func (s *Conn) Mail(from string, opts *smtp.MailOptions) error {
	s.From = from
	s.MailOpts = opts
	return nil
}

func (s *Conn) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.RcptOpts = opts
	s.To = append(s.To, to)
	return nil
}

// Data is the body - typically text format (binary encoding is also possible)
// Standard requires MIME - but malicious clients may send random stuff.
func (s *Conn) Data(r io.Reader) error {
	// TODO: limit readers to a max size
	if b, err := io.ReadAll(r); err != nil {
		return err
	} else {
		log.Println("Mail from:", s.From, s.MailOpts)
		log.Println("Rcpt to:", s.To, s.RcptOpts)
		log.Println("Data:", string(b))


	}
	return nil
}

func (s *Conn) Reset() {}

func (s *Conn) Logout() error {
	return nil
}

// ExampleServer runs an example SMTP server.
//
// It can be tested manually with e.g. netcat:
//
//	> netcat -C localhost 1025
//	EHLO localhost
//	MAIL FROM:<root@nsa.gov>
//	RCPT TO:<root@gchq.gov.uk>
//	DATA
//	Hey <3
//	.
func New() *Server {
	be := &Server{
		Domain: "localhost",
		Addr: ":1025",
	}

	// 587 is the normal port for 'submission' from a MUA, usually with SASL and SMTP
	// and authn. This is not such a server.
	// 465 - TLS
	// 2525 is also used - if 25 is blocked


	// mail headers may include 'signed by' and 'encrypted' headers.

	// First Received (in front of the original message) looks like:
	//Received: from github.com (hubbernetes-node-769ff3e.va3-iad.github.net [10.48.158.14]) by smtp.github.com (Postfix) with ESMTPA id F38368C03BB for <costin@gmail.com>; Sat, 25 Jan 2025 01:37:10 -0800 (PST)
	//DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=github.com; s=pf2023; t=1737797831; bh=5I11RnuwRNuVxvuLM+BJGlYkLD/1rzXuFxpyxeSZHBU=; h=Date:From:Reply-To:To:Cc:Subject:List-ID:List-Archive:List-Post:
	//	 List-Unsubscribe:List-Unsubscribe-Post:From; b=G4aufH8K3R1JjSDk5x12laZVP4MnwR502JdqIt2SmT5dpu4XgILXjbkr+0Tt+Wd5j
	//	 Im/dFdqTVChFtv88OpnLFk+cBRuybfksjNKrVk6EewC1OTn8y3cCY6YaRKV9twlwUB
	//	 sPAoJcDZjWvtK+Cgk4yT7hBTiH45K4pjmSQ4ioss=

	// Authentication-Results: mx.google.com;
	//       dkim=pass header.i=@github.com header.s=pf2023 header.b=G4aufH8K;
	//       spf=pass (google.com: domain of noreply@github.com designates 192.30.252.203 as permitted sender) smtp.mailfrom=noreply@github.com;
	//       dmarc=pass (p=REJECT sp=REJECT dis=NONE) header.from=github.com

	// SPF is also used to verify the IP address for the domain
	// ESMTPS indicates secure SMTP

	// ARC is for intermediaries like a mailing list.
	// ARC-Seal: i=1; a=rsa-sha256; t=1737797831; cv=none;
	//        d=google.com; s=arc-20240605;
	//        b=VRVwXu+hUS7pNf/fj9n6xyhKybcxzPLpxZuHnNmbRnuf8FLDbqsp5pU/VcXQwGot0k
	//         7rnLaDi9szlp4ahtPeZgcFMd+1WqtsWFUi86IlUka7p1Kup38pP7rqMnLC0f9wLDLWlU
	//         DTobOX0kIok1gZpM8ah/fCA9Eyld8UAnovJaM+36dV7ApY0jbx7CAPJs3zxqrlNiJ6yc
	//         T0iMsFa+K6i0hEdrFWu1xdGkRH1YfMi01EBBiAbbO7vNDKv9RW8sfcnIAuzXE9/OTk5F
	//         Ccv5GPGonA/bnsAvE8vOs763ybsvASOqTQyvNlbNN042iK+azdkfXLMx8POJT6P05Lki
	//         SQlw==
	//ARC-Message-Signature: i=1; a=rsa-sha256; c=relaxed/relaxed; d=google.com; s=arc-20240605;
	//        h=destinations:list-unsubscribe-post:list-unsubscribe:list-post
	//         :list-archive:list-id:precedence:content-transfer-encoding
	//         :mime-version:subject:message-id:cc:to:reply-to:from:date
	//         :dkim-signature;
	//        bh=5I11RnuwRNuVxvuLM+BJGlYkLD/1rzXuFxpyxeSZHBU=;
	//        fh=6orZAP4xFrpz020hA+CgB7kcfEydzk1QdddRNnm7Zvg=;
	//        b=cpBJl9SN3xB7ro0wEdWje1y289/pi7yi+OJ+hBWZZEXKKxJMeKYfKqFSoe0/7s/+Fq
	//         DoihW8i39kZFyaZD8lQIKVJ7+4owudKLSFirc0eDlnVUkaUp1B0QmmJdBmO5QdfYTJgp
	//         oIEKUpn9YxjGlotdbowWgN+OzC8rMdafNXZmtgIQHyKz37Wt5bsf0qzilKhLL8MC1mZw
	//         aYlRjrpGw37Xb57Prq6L7TgHN4hni1U+lpZV/+lzgrIeBJzSw+sPtUVwH9J7I0ahLOVT
	//         AK8CKu1NNYquB95RdAbiZtZZ+KVCxG1q++SiG2MolDogjaqKuJ+HuTU6FiFHL4Bfrowb
	//         s4Aw==;
	//        dara=google.com
	//ARC-Authentication-Results: i=1; mx.google.com;
	//       dkim=pass header.i=@github.com header.s=pf2023 header.b=G4aufH8K;
	//       spf=pass (google.com: domain of noreply@github.com designates 192.30.252.203 as permitted sender) smtp.mailfrom=noreply@github.com;
	//       dmarc=pass (p=REJECT sp=REJECT dis=NONE) header.from=github.com

	//

	return be
}

func (be *Server) Start(ctx context.Context) error {
	s := smtp.NewServer(be)

	s.Addr = s.Addr
	s.Domain = be.Domain

	s.WriteTimeout = 10 * time.Second
	s.ReadTimeout = 10 * time.Second

	s.MaxMessageBytes = 1024 * 1024
	s.MaxRecipients = 50

	s.AllowInsecureAuth = true

	s.EnableBINARYMIME = true

	s.Debug = os.Stdout

	be.s = s

	dir := maildir.Dir(".")
	be.dir = dir

	var err error
	if be.NetListener == nil {
		be.NetListener, err = net.Listen("tcp", s.Addr)
		if err != nil {
			return err
		}
	}

	log.Println("Starting server at", s.Addr)
	go func() {
		err = be.s.Serve(be.NetListener)
		if err != nil {
			slog.Error("SMTP-Serve", "err", err)
		}
	}()
	return nil
}
