//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package logyml

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/adriansr/nwdevice2filebeat/layout"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/adriansr/nwdevice2filebeat/output"
	"github.com/adriansr/nwdevice2filebeat/parser"
)

const (
	LogParserVersion = "1.0"
	license          = `#  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
#  or more contributor license agreements. Licensed under the Elastic License;
#  you may not use this file except in compliance with the Elastic License.
`
)

type logYml struct {
	tmpFile *os.File
}

func init() {
	instance := new(logYml)
	output.Registry.MustRegister("yaml", instance)
	output.Registry.MustRegister("yml", instance)
}

func (l *logYml) Settings() config.PipelineSettings {
	return config.PipelineSettings{
		// The Yaml format supports alternatives, no need to convert to dissect.
		Dissect: false,
		// Payload fields are required.
		StripPayload: false,
	}
}

func (l *logYml) Generate(parser parser.Parser) (err error) {
	l.tmpFile, err = ioutil.TempFile("", "pipeline-*.yml")
	if err != nil {
		return err
	}
	defer l.tmpFile.Close()
	cw := output.NewCodeWriter(l.tmpFile, "\t")
	cw.Raw(license)

	file := logParserFile{
		Header: fileHeader{
			Version:  LogParserVersion,
			Revision: parser.Version.Revision,
		},
		Description: fileDescription{
			Name:        parser.Description.Name,
			DisplayName: parser.Description.DisplayName,
			Type:        parser.Description.Group,
		},
		Mappings: transformMappings(parser.ValueMapsByName),
	}
	generate(parser.Root, "")
	bytes, err := yaml.Marshal(file)
	if err != nil {
		cw.Err(err)
	}
	cw.RawBytes(bytes)
	return cw.Finalize()
}

func (l *logYml) Populate(lyt *layout.Generator) (err error) {
	err = lyt.SetVar("extra_processors", fmt.Sprintf(`
- logparser:
    files:
    - ((getvar "basedir"))/((relpath "rel.dir" "config.dir"))/pipeline.yml
`))
	if err != nil {
		return err
	}
	return lyt.AddFile("__config.dir__/pipeline.yml", layout.Move{
		Path: l.tmpFile.Name(),
	})
}

func (l *logYml) OutputFile() string {
	return l.tmpFile.Name()
}

func generate(node parser.Operation, path string) {
	cur := fmt.Sprintf("%s/%T", path, node)
	if len(node.Children()) > 0 {
		for idx, child := range node.Children() {
			generate(child, fmt.Sprintf("%s[%d]", cur, idx))
		}
	} else {
		log.Println(cur)
	}
}

type logParserFile struct {
	Header      fileHeader `yaml:"logparser"`
	Description fileDescription
	Mappings    map[string]interface{}
	Headers     []match          `yaml:"one_of"`
	Messages    map[string]match `yaml:"by_key"`
}

type fileHeader struct {
	Version  string
	Revision string
}

type fileDescription struct {
	Name        string
	DisplayName string `yaml:"display_name"`
	Type        string
}

type mapping struct {
	Mappings map[string]interface{}
	Default  interface{} `yaml:",omitempty"`
}

type fieldRef struct {
	Field string
}

type match struct {
}

func transformValue(in parser.Operation) interface{} {
	switch v := in.(type) {
	case parser.Constant:
		return v.Value()
	case parser.Field:
		return fieldRef{Field: v.Name}
	default:
		// TODO: error handling
		return errors.Errorf("<unknown type %T in transformValue>", v)
	}
}

func transformMappings(in map[string]*parser.ValueMap) map[string]interface{} {
	output := make(map[string]interface{}, len(in))
	for vmapName, vmap := range in {
		m := mapping{
			Mappings: make(map[string]interface{}, len(vmap.Mappings)),
		}
		if vmap.Default != nil {
			m.Default = transformValue(*vmap.Default)
		}
		for key, idx := range vmap.Mappings {
			m.Mappings[key] = transformValue(vmap.Nodes[idx])
		}
		output[vmapName] = m
	}
	return output
}
