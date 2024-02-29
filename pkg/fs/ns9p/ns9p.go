package ns9p

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/fs"
	"github.com/knusbaum/go9p/proto"
)

type NS9P struct {
	UID string
	GID string

	Address string
}

func (p *NS9P) InitNS9PS() error {
	if p.UID == "" {
		p.UID = "build"
	}
	if p.GID == "" {
		p.GID = p.UID
	}
	utilFS, root := fs.NewFS("build", "build", 0777)

	events := fs.NewStaticFile(utilFS.NewStat("events", "build", "build", 0444), []byte{})
	root.AddChild(events)
	root.AddChild(
		WrapEvents(events, fs.NewDynamicFile(utilFS.NewStat("time", "glenda", "glenda", 0444),
			func() []byte {
				return []byte(time.Now().String() + "\n")
			},
		)),
	)
	root.AddChild(
		WrapEvents(events, &fs.WrappedFile{
			File: fs.NewBaseFile(utilFS.NewStat("random", "glenda", "glenda", 0444)),
			ReadF: func(fid uint64, offset uint64, count uint64) ([]byte, error) {
				bs := make([]byte, count)
				rand.Reader.Read(bs)
				return bs, nil
			},
		}),
	)

	// Post a local service - i.e. create a file descriptor,
	// That is using NAMESPACE env or /tmp/ns.USER.str/
	go go9p.PostSrv("utilfs", utilFS.Server())

	return go9p.Serve(p.Address, utilFS.Server())
}

func addEvent(f *fs.StaticFile, s string) {
	f.Lock()
	defer f.Unlock()
	f.Data = append(f.Data, []byte(s+"\n")...)
}

func WrapEvents(evFile *fs.StaticFile, f fs.File) fs.File {
	fname := f.Stat().Name
	return &fs.WrappedFile{
		File: f,
		OpenF: func(fid uint64, omode proto.Mode) error {
			addEvent(evFile, fmt.Sprintf("Open %s: mode: %d", fname, omode))
			return f.Open(fid, omode)
		},
		ReadF: func(fid uint64, offset uint64, count uint64) ([]byte, error) {
			addEvent(evFile, fmt.Sprintf("Read %s: offset %d, count %d", fname, offset, count))
			return f.Read(fid, offset, count)
		},
		WriteF: func(fid uint64, offset uint64, data []byte) (uint32, error) {
			addEvent(evFile, fmt.Sprintf("Write %s: offset %d, data %d bytes", fname, offset, len(data)))
			return f.Write(fid, offset, data)
		},
		CloseF: func(fid uint64) error {
			addEvent(evFile, fmt.Sprintf("Close %s", fname))
			return f.Close(fid)
		},
	}
}
