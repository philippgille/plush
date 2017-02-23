package render

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"github.com/gobuffalo/plush"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/buffalo/render"
	"github.com/pkg/errors"
	"github.com/shurcooL/github_flavored_markdown"
)

type templateRenderer struct {
	*Engine
	contentType string
	names       []string
}

func (s templateRenderer) ContentType() string {
	return s.contentType
}

func (s templateRenderer) Render(w io.Writer, data render.Data) error {
	var yield template.HTML
	var err error
	for _, name := range s.names {
		yield, err = s.execute(name, plush.NewContextWith(data))
		if err != nil {
			err = errors.Errorf("error rendering %s:\n%+v", name, err)
			return err
		}
		data["yield"] = yield
	}
	_, err = w.Write([]byte(yield))
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s templateRenderer) execute(name string, data *plush.Context) (template.HTML, error) {
	source, err := s.source(name)
	if err != nil {
		return "", err
	}

	err = source.Helpers.Add("partial", func(name string, help plush.HelperContext) (template.HTML, error) {
		p, err := s.partial(name, help.Context)
		if err != nil {
			return template.HTML(fmt.Sprintf("<pre>%s: %s</pre>", name, err.Error())), err
		}
		return p, nil
	})
	if err != nil {
		return template.HTML(fmt.Sprintf("<pre>%s: %s</pre>", name, err.Error())), err
	}

	yield, err := source.Exec(data)
	if err != nil {
		return template.HTML(fmt.Sprintf("<pre>%s: %s</pre>", name, err.Error())), err
	}
	return template.HTML(yield), nil
}

func (s templateRenderer) source(name string) (*plush.Template, error) {
	var t *plush.Template
	var ok bool
	var err error
	if s.CacheTemplates {
		if t, ok = s.templateCache[name]; ok {
			return t.Clone(), nil
		}
	}
	b, err := s.Resolver().Read(filepath.Join(s.TemplatesPath, name))
	if err != nil {
		return nil, errors.WithStack(fmt.Errorf("could not find template: %s", name))
	}
	if strings.ToLower(filepath.Ext(name)) == ".md" {
		b = github_flavored_markdown.Markdown(b)
		// unescape quotes so raymond can parse the file correctly.
		b = bytes.Replace(b, []byte("&lt;"), []byte("<"), -1)
		b = bytes.Replace(b, []byte("&gt;"), []byte(">"), -1)
	}
	source := string(b)
	t, err = plush.Parse(source)
	if err != nil {
		return t, errors.Errorf("Error parsing %s: %+v", name, errors.WithStack(err))
	}

	err = t.Helpers.AddMany(s.Helpers)
	if err != nil {
		return nil, err
	}
	if s.CacheTemplates {
		s.templateCache[name] = t
	}
	return t.Clone(), err
}

func (s templateRenderer) partial(name string, data *plush.Context) (template.HTML, error) {
	d, f := filepath.Split(name)
	name = filepath.Join(d, "_"+f)
	return s.execute(name, data)
}

// Template renders the named files using the specified
// content type and the github.com/aymerick/raymond
// package for templating. If more than 1 file is provided
// the second file will be considered a "layout" file
// and the first file will be the "content" file which will
// be placed into the "layout" using "{{yield}}".
func Template(c string, names ...string) Renderer {
	e := New(Options{})
	return e.Template(c, names...)
}

// Template renders the named files using the specified
// content type and the github.com/aymerick/raymond
// package for templating. If more than 1 file is provided
// the second file will be considered a "layout" file
// and the first file will be the "content" file which will
// be placed into the "layout" using "{{yield}}".
func (e *Engine) Template(c string, names ...string) Renderer {
	return templateRenderer{
		Engine:      e,
		contentType: c,
		names:       names,
	}
}
