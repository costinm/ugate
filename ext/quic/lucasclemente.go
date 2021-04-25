package quic

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/costinm/ugate"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/quicvarint"
)

// Copied from the original library, private.

type errorCode quic.ErrorCode

const (
	errorNoError              errorCode = 0x100
	errorGeneralProtocolError errorCode = 0x101
	errorInternalError        errorCode = 0x102
	errorStreamCreationError  errorCode = 0x103
	errorClosedCriticalStream errorCode = 0x104
	errorFrameUnexpected      errorCode = 0x105
	errorFrameError           errorCode = 0x106
	errorExcessiveLoad        errorCode = 0x107
	errorIDError              errorCode = 0x108
	errorSettingsError        errorCode = 0x109
	errorMissingSettings      errorCode = 0x10a
	errorRequestRejected      errorCode = 0x10b
	errorRequestCanceled      errorCode = 0x10c
	errorRequestIncomplete    errorCode = 0x10d
	errorMessageError         errorCode = 0x10e
	errorConnectError         errorCode = 0x10f
	errorVersionFallback      errorCode = 0x110
)

type frame interface{}

func parseNextFrame(br *ugate.BufferedStream) (frame, error) {
	t, err := quicvarint.Read(br)
	if err != nil {
		return nil, err
	}
	l, err := quicvarint.Read(br)
	if err != nil {
		return nil, err
	}

	switch t {
	case 0x0:
		return &dataFrame{Length: l}, nil
	case 0x1:
		return &headersFrame{Length: l}, nil
	case 0x4:
		return parseSettingsFrame(br, l)
	case 0x3: // CANCEL_PUSH
		fallthrough
	case 0x5: // PUSH_PROMISE
		fallthrough
	case 0x7: // GOAWAY
		fallthrough
	case 0xd: // MAX_PUSH_ID
		fallthrough
	case 0xe: // DUPLICATE_PUSH
		fallthrough
	default:
		// skip over unknown frames
		if _, err := io.CopyN(ioutil.Discard, br, int64(l)); err != nil {
			return nil, err
		}
		return parseNextFrame(br)
	}
}

type dataFrame struct {
	Length uint64
}

func (f *dataFrame) Write(b *bytes.Buffer) {
	quicvarint.Write(b, 0x0)
	quicvarint.Write(b, f.Length)
}

type headersFrame struct {
	Length uint64
}

func (f *headersFrame) Write(b *bytes.Buffer) {
	quicvarint.Write(b, 0x1)
	quicvarint.Write(b, f.Length)
}

const settingDatagram = 0x276

type settingsFrame struct {
	Datagram bool
	other    map[uint64]uint64 // all settings that we don't explicitly recognize
}

func parseSettingsFrame(r io.Reader, l uint64) (*settingsFrame, error) {
	if l > 8*(1<<10) {
		return nil, fmt.Errorf("unexpected size for SETTINGS frame: %d", l)
	}
	buf := make([]byte, l)
	if _, err := io.ReadFull(r, buf); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, io.EOF
		}
		return nil, err
	}
	frame := &settingsFrame{}
	b := bytes.NewReader(buf)
	var readDatagram bool
	for b.Len() > 0 {
		id, err := quicvarint.Read(b)
		if err != nil { // should not happen. We allocated the whole frame already.
			return nil, err
		}
		val, err := quicvarint.Read(b)
		if err != nil { // should not happen. We allocated the whole frame already.
			return nil, err
		}

		switch id {
		case settingDatagram:
			if readDatagram {
				return nil, fmt.Errorf("duplicate setting: %d", id)
			}
			readDatagram = true
			if val != 0 && val != 1 {
				return nil, fmt.Errorf("invalid value for H3_DATAGRAM: %d", val)
			}
			frame.Datagram = val == 1
		default:
			if _, ok := frame.other[id]; ok {
				return nil, fmt.Errorf("duplicate setting: %d", id)
			}
			if frame.other == nil {
				frame.other = make(map[uint64]uint64)
			}
			frame.other[id] = val
		}
	}
	return frame, nil
}

func (f *settingsFrame) Write(b *bytes.Buffer) {
	quicvarint.Write(b, 0x4)
	var l int
	for id, val := range f.other {
		l += Len(id) + Len(val)
	}
	if f.Datagram {
		l += Len(settingDatagram) + Len(1)
	}
	quicvarint.Write(b, uint64(l))
	if f.Datagram {
		quicvarint.Write(b, settingDatagram)
		quicvarint.Write(b, 1)
	}
	for id, val := range f.other {
		quicvarint.Write(b, id)
		quicvarint.Write(b, val)
	}
}

// taken from the QUIC draft
const (
	maxVarInt1 = 63
	maxVarInt2 = 16383
	maxVarInt4 = 1073741823
	maxVarInt8 = 4611686018427387903
)

// Len determines the number of bytes that will be needed to write a number
func Len(i uint64) int {
	if i <= maxVarInt1 {
		return 1
	}
	if i <= maxVarInt2 {
		return 2
	}
	if i <= maxVarInt4 {
		return 4
	}
	if i <= maxVarInt8 {
		return 8
	}
	// Don't use a fmt.Sprintf here to format the error message.
	// The function would then exceed the inlining budget.
	panic(struct {
		message string
		num     uint64
	}{"value doesn't fit into 62 bits: ", i})
}

