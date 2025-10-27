package tmpl

import (
	"github.com/Masterminds/sprig/v3"
	"html/template"
  )
  
  
  
// Tmpl is a wrapper around a Golang template.
type Tmpl struct {
}

// TmplState holds the state and methods exposed to one instance of a template rendering.
type TmplState struct {

}

func New() *Tmpl {
// This example illustrates that the FuncMap *must* be set before the
  // templates themselves are loaded.
  tpl := template.Must(
	template.New("base").Funcs(sprig.FuncMap()).ParseGlob("*.html")
  )
  	
  return &Tmpl{tpl: tpl}
}

func (t *Tmpl) Render() string {
  return ""
}