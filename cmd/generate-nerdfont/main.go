package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"text/template"
)

const estimatedGlyphCount = 12_000

var noCache bool

func main() {
	flag.BoolVar(&noCache, "no-cache", noCache, "disable caching")
	pkgname := flag.String("pkg", "nerdfonts", "set the output package name")
	outpath := flag.String("out", "-", "nerdfont output file")
	genMappingFn := flag.Bool("generate-mapping-function", false, "generate a function to map class names to icons")
	genList := flag.Bool("generate-glyph-list", false, "generate a list of all glyphs")
	flag.Parse()

	validateStringFlag(pkgname, "pkg")
	validateStringFlag(outpath, "out")
	out := getOut(*outpath)
	defer verboseClose(out)

	metadata, glist, err := glyphsList()
	if err != nil {
		log.Fatal(err)
	}
	templ, err := newTempl()
	if err != nil {
		log.Fatal(err)
	}
	err = templ.Execute(out, Data{
		Package:         *pkgname,
		Metadata:        *metadata,
		Glyphs:          glist,
		MappingFunction: *genMappingFn,
		GenList:         *genList,
	})
	if err != nil {
		log.Fatal(err)
	}
}

type Data struct {
	Package  string
	Glyphs   []Glyph
	Metadata GlyphsMetadata

	// bool flags

	MappingFunction bool
	GenList         bool
}

func newTempl() (*template.Template, error) {
	t, err := template.New("nerdfonts").Funcs(template.FuncMap{
		"ConstName": func(name string) string { return classToConstName(name) },
	}).Parse(`// Code generated. DO NOT EDIT.

// Package {{ .Package }} holds nerd font glyfs.
//
// Nerdfonts version {{ .Metadata.Version }} released at {{ .Metadata.Date }}
package {{ .Package }}

const (
{{- range .Glyphs }}
	// {{ .ConstName }} maps to "{{ .ID }}" (0x{{ .Code }}).{{- if .Alias }} Alias of "{{ .Alias }}".
	{{ .ConstName }} = {{ .Alias | ConstName }}
	{{- else }}
	{{ .ConstName }} = "{{ .Icon }}"
	{{- end }}
{{- end }}
)
{{- if .MappingFunction }}
func IconFromClassName(name string) (string, bool) {
	switch name {
{{- range .Glyphs }}
	case "{{ .ID }}":
		return {{ .ConstName }}, true
{{- end }}
	default:
		return "", false
	}
}
{{ end -}}
{{- if .GenList }}

// Glyph represents a nerdfont glyph.
type Glyph struct {
	Icon  string
	Class string
	Code   uint32
}

// Glyphs is a list of all nerdfont glyphs.
var Glyphs = [...]Glyph{
{{- range .Glyphs }}
	{Icon: {{ .ConstName }}, Class: "{{ .ID }}", Code: {{ .HexCode | printf "0x%x" }}},
{{- end }}
}
{{ end -}}
`)
	return t, err
}

func getOut(outpath string) io.WriteCloser {
	if outpath == "-" {
		return NopWriter{os.Stdout}
	} else {
		dir := filepath.Dir(outpath)
		_ = os.MkdirAll(dir, 0755)
		outfile, err := os.OpenFile(outpath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		return outfile
	}
}

func validateStringFlag(value *string, name string) {
	if value == nil || len(*value) == 0 {
		log.Fatalf("-%s is required", name)
	}
}

type NopWriter struct{ io.Writer }

func (NopWriter) Close() error { return nil }

func verboseClose(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}
