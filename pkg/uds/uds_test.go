package uds

import (
	"context"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"

	msgs "github.com/costinm/ugate/webpush"
)

func TestUDS(t *testing.T) {
	//utel.InitDefaultHandler(nil)

	fn := "testUDS"
	us, err := NewServer(fn, msgs.DefaultMux)
	defer os.Remove(fn)
	if err != nil {
		t.Error("Failed to listen", err)
		return
	}
	go us.Start()

	msgs.DefaultMux.AddHandler("srv", msgs.HandlerCallbackFunc(func(ctx context.Context, cmdS string, meta map[string]string, data []byte) {
		log.Println(cmdS, meta, data)
	}))

	cc, err := Dial(fn, msgs.DefaultMux, map[string]string{})
	if err != nil {
		t.Error("Failed to Redial", err)
		return
	}
	go cc.HandleStream()
	msgs.DefaultMux.AddHandler("client", msgs.HandlerCallbackFunc(func(ctx context.Context, cmdS string, meta map[string]string, data []byte) {
		log.Println(cmdS, meta, data)
	}))

	cc.SendMessageDirect("srv 1", nil, nil)

	//us.SendMessageDirect("client", nil, nil)

}

func TestParse(t *testing.T) {

	for _, inp := range []struct {
		in      string
		cmd     string
		expData string
		expMeta map[string]string
	}{
		{in: "test", cmd: "test"},
		{in: "test cmd", cmd: "test"},
		{in: "test cmd\n\ndata", cmd: "test", expData: "data"},
		{in: "test cmd\na:b\n\ndata", cmd: "test", expData: "data", expMeta: map[string]string{"a": "b"}},
	} {
		m := []byte(inp.in)
		cmdS, meta, data, _ := ParseMessage(m, len(m))

		argv := strings.Split(cmdS, " ")
		if argv[0] != inp.cmd {
			t.Error(cmdS)
		}
		if inp.expData == "" && len(data) > 0 {
			t.Error("Unexpected payload ", data)
		} else if string(data) != inp.expData {
			t.Error("Expecting ", inp.expData, " got ", string(data))
		}

		if inp.expMeta == nil {
			if len(meta) > 0 {
				t.Error("Unexpected meta ", meta)
			}
		} else {
			if !reflect.DeepEqual(inp.expMeta, meta) {
				t.Error("Expecting meta ", inp.expMeta, "got", meta)
			}
		}
	}

}
