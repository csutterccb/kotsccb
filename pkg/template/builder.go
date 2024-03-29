package template

import (
	"bytes"
	"regexp"
	"strconv"
	"text/template"

	"github.com/pkg/errors"
)

var (
	templateNotDefinedRegexp = regexp.MustCompile(`template.*not defined$`)
)

type Builder struct {
	Ctx    []Ctx
	Functs template.FuncMap
}

func (b *Builder) AddCtx(ctx Ctx) {
	b.Ctx = append(b.Ctx, ctx)
}

func (b *Builder) String(text string) (string, error) {
	if text == "" {
		return "", nil
	}
	return b.RenderTemplate(text, text)
}

func (b *Builder) Bool(text string, defaultVal bool) (bool, error) {
	if text == "" {
		return defaultVal, nil
	}

	value, err := b.RenderTemplate(text, text)
	if err != nil {
		return defaultVal, errors.Wrap(err, "failed to render bool template")
	}

	// If the template didn't parse (turns into an empty string), then we should
	// return the default
	if value == "" {
		return defaultVal, nil
	}

	result, err := strconv.ParseBool(value)
	if err != nil {
		// for now we are assuming default value if we fail to parse
		return defaultVal, nil
	}

	return result, nil
}

func (b *Builder) Int(text string, defaultVal int64) (int64, error) {
	if text == "" {
		return defaultVal, nil
	}

	value, err := b.RenderTemplate(text, text)
	if err != nil {
		return defaultVal, errors.Wrap(err, "failed to render int template")
	}

	// If the template didn't parse (turns into an empty string), then we should
	// return the default
	if value == "" {
		return defaultVal, nil
	}

	result, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		// for now we are assuming default value if we fail to parse
		return defaultVal, nil
	}

	return result, nil
}

func (b *Builder) Uint(text string, defaultVal uint64) (uint64, error) {
	if text == "" {
		return defaultVal, nil
	}

	value, err := b.RenderTemplate(text, text)
	if err != nil {
		return defaultVal, errors.Wrap(err, "failed to render uint template")
	}

	// If the template didn't parse (turns into an empty string), then we should
	// return the default
	if value == "" {
		return defaultVal, nil
	}

	result, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		// for now we are assuming default value if we fail to parse
		return defaultVal, nil
	}

	return result, nil
}

func (b *Builder) Float64(text string, defaultVal float64) (float64, error) {
	if text == "" {
		return defaultVal, nil
	}

	value, err := b.RenderTemplate(text, text)
	if err != nil {
		return defaultVal, errors.Wrap(err, "failed to render float template")
	}

	// If the template didn't parse (turns into an empty string), then we should
	// return the default
	if value == "" {
		return defaultVal, nil
	}

	result, err := strconv.ParseFloat(value, 64)
	if err != nil {
		// for now we are assuming default value if we fail to parse
		return defaultVal, nil
	}

	return result, nil
}

func (b *Builder) BuildFuncMap() template.FuncMap {
	if b.Functs == nil {
		b.Functs = template.FuncMap{}
	}

	funcMap := b.Functs
	for _, ctx := range b.Ctx {
		for name, fn := range ctx.FuncMap() {
			funcMap[name] = fn
		}
	}
	return funcMap
}

func (b *Builder) GetTemplate(name, text string) (*template.Template, error) {
	tmpl, err := template.New(name).Delims("{{repl ", "}}").Funcs(b.BuildFuncMap()).Parse(text)
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

func (b *Builder) RenderTemplate(name string, text string) (string, error) {
	tmpl, err := b.GetTemplate(name, text)
	if err != nil {
		return "", errors.Wrap(err, "failed to get template")
	}

	var contents bytes.Buffer
	if err := tmpl.Execute(&contents, nil); err != nil {
		return "", errors.Wrap(err, "failed to execute template")
	}

	return contents.String(), nil
}
