package gotmpl

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"strings"
	"text/template"

	"github.com/go-task/slim-sprig/v3"
	"sigs.k8s.io/yaml"
)

// Support go templates as runbooks.
// The go templates can use '{{ tool NAME key1 value1 key2 value2... }} to call (MCP) tools.

// Libraries used:
// "github.com/go-task/slim-sprig/v3" - minimal, no external deps
// sprig - medium
// https://coveooss.github.io/gotemplate/docs/functions_reference/all_functions/

type TemplateCtx struct {
	// The template id
	Id       string `json:"id"`
	Template string `json:"template"`

	Params map[string]any `json:"params"`

	template *template.Template
}

func (tc *TemplateCtx) Post(url string, tmpl string, headers ...string) (any, error) {
	r, err := tc.HttpRequest("POST", url, tmpl, headers...)
	if err != nil {
		return nil, err
	}
	return tc.HttpDo(r.(*http.Request))
}

func (tc *TemplateCtx) HttpRequest(method, url string, tmpl string, headers ...string) (any, error) {
	body := bytes.NewReader([]byte(tmpl))
	r, err := http.NewRequest(method, url, body)
	for i := 0; i < len(headers)-1; i += 2 {
		r.Header.Add(headers[2*i], headers[2*i+1])
	}
	if err != nil {
		return nil, err
	}
	return r, err
}

func (tc *TemplateCtx) HttpDo(r *http.Request) (any, error) {
	res, err := http.DefaultClient.Do(r)

	return res, err
}

func (tc *TemplateCtx) Read(r io.ReadCloser) (any, error) {
	return ioutil.ReadAll(r)
}

func New() *TemplateCtx {
	return &TemplateCtx{
		Params: make(map[string]any),
	}
}

func (tc *TemplateCtx) Run(ctx context.Context) (any, error) {
	if tc.template == nil {
		if tc.Template == "" {
			var fsroot fs.FS
			fsroot = static.Content
			fsr := ctx.Value("fs")
			if lfsr, ok := fsr.(fs.FS); ok {
				fsroot = lfsr
			}
			f, err := fsroot.Open(tc.Id)
			if err != nil {
				return nil, err
			}
			bd, err := io.ReadAll(f)
			if err != nil {
				return nil, err
			}
			tc.Template = string(bd)
		}
		t, err := template.New(tc.Id).Funcs(sprig.FuncMap()).
			Funcs(map[string]any{
				"post":    tc.Post,
				"read":    tc.Read,
				"httpDo":  tc.HttpDo,
				"httpReq": tc.HttpRequest,
				"include": func(name string, p any) (any, error) {
					// TODO: if name starts with "/" - sue the fs to compile a new template, cache
					// Support /foo/name.tmpl#name
					// All files relative to base.
					w := &bytes.Buffer{}
					ta := tc.template.Templates()
					for _, tt := range ta {
						if tt.Name() == name {
							err := tt.Execute(w, p)
							if err != nil {
								return nil, err
							}
							return w.String(), nil
						}
					}
					return nil, nil
				},
			}).Parse(tc.Template)
		if err != nil {
			return nil, err
		}
		tc.template = t
	}

	w := &bytes.Buffer{}
	tc.template.Execute(w, tc)

	// expect the result to be yaml followed by markdown
	// split w by "---"

	o := Observation{
		Meta: make(map[string]any),
	}

	str := string(w.Bytes())

	parts := strings.SplitN(str, "---", 3)
	if len(parts) == 3 {
		str = parts[2]
		err := yaml.Unmarshal([]byte(parts[1]), o.Meta)
		if err != nil {
			return nil, err
		}
	}
	if strings.HasPrefix(str, "{") {
		// Unstructured data only - not a full observation
		json.Unmarshal(w.Bytes(), o.Meta)
	} else {
		o.Observation = str
	}

	return o, nil

}

type Observation struct {
	Meta        map[string]any
	Observation string
}
