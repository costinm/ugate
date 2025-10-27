package gotmpl

import (
	"context"
	"io/ioutil"
	"testing"
)


func TestTmpl(t *testing.T)  {
	d, err := ioutil.ReadFile("testdata/example.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	tmpl := New()
	tmpl.Template = string(d)

	ctx := context.Background()
	res, err := tmpl.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(res)
}
