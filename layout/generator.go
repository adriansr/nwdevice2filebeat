//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package layout

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/adriansr/nwdevice2filebeat/util"
)

const (
	templatesDir = "layout"
	templateExt  = ".tpl"
	dirExt       = ".dir"

	// Use different template delimiters as some files being output already
	// contain templates in them using the default `{{ }}` (config.yml)
	// or `{< >}` (ingest pipelines). Other formats (asciidoc) don't like
	// curly braces too much.
	templatesDelimLeft  = "(("
	templatesDelimRight = "))"

	pathDelimLeft  = "__"
	pathDelimRight = "__"
)

type Generator struct {
	vars    Vars
	dynVars map[string]string
	files   map[string]FileWriter
	inlines map[string]FileWriter
	dirs    map[string]string
	repl    pathReplacements
	funcs   template.FuncMap
}

type Vars struct {
	LogParser     parser.Parser
	Categories    []string
	DisplayName   string
	Group         string
	Module        string
	Fileset       string
	Icon          string
	Product       string
	Vendor        string
	Version       string
	Port          uint16
	GeneratedTime time.Time
}

type pathReplacements map[string]string

var validPathRepl = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func (v Vars) pathReplacements() pathReplacements {
	repl := make(pathReplacements)
	value := reflect.ValueOf(v)
	vType := reflect.TypeOf(v)
	for i := 0; i < value.NumField(); i++ {
		fld := value.Field(i)
		fldType := vType.Field(i)
		if fld.Kind() == reflect.String && validPathRepl.MatchString(fld.String()) {
			expr := pathDelimLeft + strings.ToLower(fldType.Name) + pathDelimRight
			repl[expr] = fld.String()
		}
	}
	return repl
}

func (repl pathReplacements) Apply(path string) string {
	for old, new := range repl {
		path = strings.ReplaceAll(path, old, new)
	}
	return path
}

func (g *Generator) doDir(name string) (result string, err error) {
	path, ok := g.dirs[name]
	if !ok {
		return "", errors.Errorf("directory %s not defined. Missing %s file", name, name)
	}
	return path, nil
}

func (g *Generator) doVar(name string) (result string, err error) {
	value, ok := g.dynVars[name]
	if !ok {
		return "", errors.Errorf("variable %s not defined", name)
	}
	tpl, err := g.newTemplate().Parse(value)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err = tpl.Execute(&sb, g.vars); err != nil {
		return "", err
	}
	//g.dynVars[name] = sb.String()

	return sb.String(), nil
}

func (g *Generator) doSet(name, value string) (empty string, err error) {
	//if _, exists := g.dynVars[name]; exists {
	//	return empty, errors.Errorf("variable %s is already defined", name)
	//}
	g.dynVars[name] = value
	return empty, nil
}

func (g *Generator) doInline(file string) (result string, err error) {
	writer, found := g.inlines[file]
	if !found {
		return "", errors.Errorf("inline file '%s' not found", file)
	}
	buffer := bytes.NewBuffer(nil)
	if err := writer.WriteFile(buffer); err != nil {
		return "", errors.Wrapf(err, "error inlining file '%s'", file)
	}
	return buffer.String(), nil
}

func (g *Generator) doRelPath(base, subpath string) (result string, err error) {
	basedir, err := g.doDir(base)
	if err != nil {
		return "", err
	}
	subdir, err := g.doDir(subpath)
	return filepath.Rel(basedir, subdir)
}

func (g *Generator) doIndent(indent string, times int, value string) (result string, err error) {
	prefix := strings.Repeat(indent, times)
	return prefix + strings.Replace(value, "\n", "\n"+prefix, -1), nil
}

func (g *Generator) doToJSON(value interface{}) (result string, err error) {
	data, err := json.Marshal(value)
	return string(data), err
}

func New(layout string, vars Vars) (*Generator, error) {
	baseDir := filepath.Join(templatesDir, layout)
	files, err := util.ListFilesRecursive(baseDir)
	if err != nil {
		return nil, errors.Wrapf(err, "error loading output template %s", layout)
	}
	gen := &Generator{
		dirs: make(map[string]string),
		vars: vars,
		repl: vars.pathReplacements(),
	}
	gen.funcs = template.FuncMap{
		"dir":     gen.doDir,
		"relpath": gen.doRelPath,
		"getvar":  gen.doVar,
		"setvar":  gen.doSet,
		"title":   strings.Title,
		"inline":  gen.doInline,
		"indent":  gen.doIndent,
		"tojson":  gen.doToJSON,
	}
	for _, sourcePath := range files {
		// Strip <templatesDir>/<template> prefix from paths.
		destPath, err := filepath.Rel(baseDir, sourcePath)
		if err != nil {
			return nil, err
		}
		switch util.FileExtension(sourcePath) {
		case templateExt:
			// Strip .tpl extension
			destPath = destPath[:len(destPath)-len(templateExt)]
			err = gen.AddFile(destPath, TemplateFile{
				Path: sourcePath,
				Tpl:  gen.newTemplate(),
				Vars: vars,
			})
			if err != nil {
				return nil, err
			}

		case dirExt:
			// When a <name>.dir file exists, store the path to the directory
			// it resides in the path replacement __<name>.dir__.
			dirName, fileName := filepath.Split(gen.repl.Apply(destPath))
			pathVar := pathDelimLeft + fileName + pathDelimRight
			gen.repl[pathVar] = dirName
			// Also store in a variable so it can be accessed in a template as
			// ((dir "name.dir")) or ((relpath "rel.dir" "name.dir"))
			gen.dirs[fileName] = dirName

		default:
			if err = gen.AddFile(destPath, Copy{Path: sourcePath}); err != nil {
				return nil, err
			}
		}
	}
	return gen, nil
}

func (g *Generator) AddFile(path string, gen FileWriter) error {
	if g.files == nil {
		g.files = make(map[string]FileWriter)
	}
	if _, exists := g.files[path]; exists {
		return errors.Errorf("output file '%s' is generated more than once", path)
	}
	g.files[path] = gen
	return nil
}

func (g *Generator) AddInlineFile(path string, gen FileWriter) error {
	if g.inlines == nil {
		g.inlines = make(map[string]FileWriter)
	}
	if _, exists := g.inlines[path]; exists {
		return errors.Errorf("output file '%s' is generated more than once", path)
	}
	g.inlines[path] = gen
	return nil
}

func (g *Generator) HasDir(name string) bool {
	_, has := g.dirs[name]
	return has
}

func (g *Generator) SetVar(name, value string) error {
	if _, found := g.dynVars[name]; found {
		return errors.Errorf("output layout variable '%s' redefined", name)
	}
	if g.dynVars == nil {
		g.dynVars = make(map[string]string)
	}
	g.dynVars[name] = value
	return nil
}

func (g *Generator) generateFile(targetDir, tplPath string, gen FileWriter) error {
	path := g.repl.Apply(tplPath)
	destPath := filepath.Join(targetDir, path)
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0777); err != nil {
		return err
	}
	destF, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return errors.Wrapf(err, "error creating file '%s'", destPath)
	}
	defer destF.Close()
	dest := util.NewCountingWriter(destF)
	if err := gen.WriteFile(dest); err != nil {
		return errors.Wrapf(err, "error generating file '%s'", destPath)
	}
	log.Printf(" - output %d bytes for file %s", dest.Count(), destPath)
	return nil
}

func (g *Generator) Build(targetDir string) error {
	info, err := os.Stat(targetDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.Errorf("output path '%s' is not a directory", targetDir)
	}
	for tplPath, gen := range g.files {
		if err = g.generateFile(targetDir, tplPath, gen); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) newTemplate() *template.Template {
	return template.New("").
		Delims(templatesDelimLeft, templatesDelimRight).
		Funcs(g.funcs)
}

type FileWriter interface {
	WriteFile(dest io.Writer) error
}

type TemplateFile struct {
	Path string
	Tpl  *template.Template
	Vars Vars
}

func (tf TemplateFile) WriteFile(dest io.Writer) (err error) {
	content, err := ioutil.ReadFile(tf.Path)
	if err != nil {
		return errors.Wrapf(err, "error reading template file '%s'", tf.Path)
	}
	tpl := tf.Tpl
	if tpl, err = tpl.Parse(string(content)); err != nil {
		return errors.Wrap(err, "parse")
	}
	if err = tpl.Execute(dest, tf.Vars); err != nil {
		return errors.Wrap(err, "execute")
	}
	return nil
}

type Copy struct {
	Path string
}

func (c Copy) WriteFile(dest io.Writer) error {
	src, err := os.Open(c.Path)
	if err != nil {
		return errors.Wrapf(err, "error opening file '%s' for reading", c.Path)
	}
	defer src.Close()
	_, err = io.Copy(dest, src)
	return err
}

type Move Copy

func (m Move) WriteFile(dest io.Writer) error {
	defer os.Remove(m.Path)
	return Copy(m).WriteFile(dest)
}
