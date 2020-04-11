//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package model

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/adriansr/nwdevice2filebeat/util"
	"github.com/pkg/errors"
	"golang.org/x/net/html/charset"
)

type Device struct {
	XMLPath                  string
	name, displayName, group string
}

// New turns a new Device from the given path path.
func NewDevice(path string) (Device, error) {
	files, err := listFilesByExtensions(path)
	if err != nil {
		return Device{}, err
	}
	log.Printf("Loaded files: %+v", files)

	xmlFiles := files[".xml"]
	// Device log parser dirs only contain one XML.
	switch len(xmlFiles) {
	case 0:
		return Device{}, errors.Errorf("device path doesn't contain a parser definition (no XML file found)")
	case 1:
	default:
		return Device{}, errors.Errorf("exactly one XML file expected in path, found=%+v", xmlFiles)
	}

	dev := Device{
		XMLPath: xmlFiles[0],
	}
	return dev, dev.load()
}

func listFilesByExtensions(dir string) (filesByExt map[string][]string, err error) {
	filesByExt = make(map[string][]string)
	return filesByExt, filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(info.Name()))
		filesByExt[ext] = append(filesByExt[ext], path)
		return nil
	})
}

type xmlPos struct {
	path string
	line uint64
	col  uint64
}

func (p xmlPos) String() string {
	return fmt.Sprintf("%s:%d:%d", p.path, p.line, p.col)
}

type stateFn func(dev *Device, token xml.Token, pos xmlPos) (xmlState, error)

func name(n string) xml.Name {
	return xml.Name{
		Local: n,
	}
}

type attrExtractor func(value string, dev *Device, pos xmlPos) error

type xmlState int

const (
	xmlStateErr xmlState = iota
	xmlStateProcInst
	xmlStateDeviceMessages
	xmlStateBody
	xmlStateHeader
	xmlStateHeaderClose
	xmlStateVersion
	xmlStateVersionClose
	xmlStateValueMap
	xmlStateValueMapClose
	//xmlStateTagValueMap
	//xmlStateTagValueMapClose
	xmlStateTagValMap
	xmlStateTagValMapClose
	xmlStateMessage
	xmlStateMessageClose
	xmlStateVarType
	xmlStateVarTypeClose
	xmlStateRegX
	xmlStateRegXClose
	xmlStateSumData
	xmlStateSumDataClose
	xmlStateEnd
)

var xmlStates = map[xmlState]stateFn{
	xmlStateProcInst:       stateProcInst,
	xmlStateBody:           stateBody,
	xmlStateDeviceMessages: deviceMessagesState,
	xmlStateHeader:         stateHeader,
	xmlStateHeaderClose:    closeTagDecoder(name("HEADER"), xmlStateBody),
	xmlStateVersion:        stateVersion,
	xmlStateVersionClose:   closeTagDecoder(name("VERSION"), xmlStateBody),
	xmlStateValueMap:       stateValueMap,
	xmlStateValueMapClose:  closeTagDecoder(name("VALUEMAP"), xmlStateBody),
	//xmlStateTagValueMap: stateTagValueMap,
	//xmlStateTagValueMapClose: closeTagDecoder(name("TAGVALUEMAP"), xmlStateBody),
	xmlStateTagValMap:      stateTagValMap,
	xmlStateTagValMapClose: closeTagDecoder(name("TAGVALMAP"), xmlStateBody),
	xmlStateMessage:        stateMessage,
	xmlStateMessageClose:   closeTagDecoder(name("MESSAGE"), xmlStateBody),
	xmlStateVarType:        stateVarType,
	xmlStateVarTypeClose:   closeTagDecoder(name("VARTYPE"), xmlStateBody),
	xmlStateRegX:           stateRegX,
	xmlStateRegXClose:      closeTagDecoder(name("REGX"), xmlStateBody),
	xmlStateSumData:        stateSumData,
	xmlStateSumDataClose:   closeTagDecoder(name("SUMDATA"), xmlStateBody),
	xmlStateEnd:            stateEnd,
}

var deviceMessagesState = tagDecoder(name("DEVICEMESSAGES"), map[xml.Name]attrExtractor{
	name("name"): func(value string, dev *Device, pos xmlPos) error {
		dev.name = value
		return nil
	},
	name("displayname"): func(value string, dev *Device, pos xmlPos) error {
		dev.displayName = value
		return nil
	},
	name("group"): func(value string, dev *Device, pos xmlPos) error {
		dev.group = value
		return nil
	},
}, xmlStateDeviceMessages, xmlStateBody)

var stateHeader = tagDecoder(name("HEADER"), map[xml.Name]attrExtractor{
	name("id1"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("id2"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("eventcategory"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("missField"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("tagval"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("functions"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("content"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("messageid"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("prioritize"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("devts"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
}, xmlStateHeader, xmlStateHeaderClose)

var stateVersion = tagDecoder(name("VERSION"), map[xml.Name]attrExtractor{
	name("xml"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("checksum"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("revision"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("device"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("enVision"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
}, xmlStateVersion, xmlStateVersionClose)

/*var stateTagValueMap = tagDecoder(name("TAGVALUEMAP"), map[xml.Name]attrExtractor{
	name("pairdelimiter"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("encapsulator"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
}, xmlStateTagValueMap, xmlStateTagValueMapClose)*/

var stateTagValMap = tagDecoder(name("TAGVALMAP"), map[xml.Name]attrExtractor{
	name("delimiter"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("valuedelimiter"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("pairdelimiter"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("escapeValueDelim"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("encapsulator"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
}, xmlStateTagValMap, xmlStateTagValMapClose)

var stateValueMap = tagDecoder(name("VALUEMAP"), map[xml.Name]attrExtractor{
	name("name"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("default"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("keyvaluepairs"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
}, xmlStateValueMap, xmlStateValueMapClose)

var stateVarType = tagDecoder(name("VARTYPE"), map[xml.Name]attrExtractor{
	name("name"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("regex"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("ignorecase"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
}, xmlStateVarType, xmlStateVarTypeClose)

var stateMessage = tagDecoder(name("MESSAGE"), map[xml.Name]attrExtractor{
	name("id1"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("id2"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("eventcategory"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("missField"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("tagval"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("functions"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("content"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("messageid"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("level"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("parse"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("parsedefvalue"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("tableid"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("summary"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
}, xmlStateMessage, xmlStateMessageClose)

var stateRegX = tagDecoder(name("REGX"), map[xml.Name]attrExtractor{
	name("name"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("parms"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("default"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("expr"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
}, xmlStateRegX, xmlStateRegXClose)

var stateSumData = tagDecoder(name("SUMDATA"), map[xml.Name]attrExtractor{
	name("bucket"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("key"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("subkey"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
	name("fields"): func(value string, dev *Device, pos xmlPos) error {
		return nil
	},
}, xmlStateSumData, xmlStateSumDataClose)

var allowedBodyTags = map[xml.Name]stateFn{
	name("HEADER"):    stateHeader,
	name("VERSION"):   stateVersion,
	name("TAGVALMAP"): stateTagValMap,
	//name("TAGVALUEMAP"):stateTagValueMap, // TODO: Check
	name("VALUEMAP"): stateValueMap,
	name("MESSAGE"):  stateMessage,
	name("VARTYPE"):  stateVarType,
	name("REGX"):     stateRegX,
	name("SUMDATA"):  stateSumData,
}

func stateBody(dev *Device, token xml.Token, pos xmlPos) (xmlState, error) {
	switch v := token.(type) {
	case xml.StartElement:
		fn, found := allowedBodyTags[v.Name]
		if !found {
			return xmlStateErr, errors.Errorf("unexpected XML tag:%s", v.Name)
		}
		return fn(dev, token, pos)

	case xml.ProcInst:
		return xmlStateErr, errors.Errorf("unexpected XML: ProcInst found while scanning for body")

	case xml.CharData, xml.Comment, xml.Directive: // ignore
		return xmlStateBody, nil

	case xml.EndElement:
		if v.Name != name("DEVICEMESSAGES") {
			return xmlStateErr, errors.Errorf("unexpected closing tag:%s", v.Name)
		}
		return xmlStateEnd, nil

	default:
		return xmlStateErr, errors.Errorf("unknown XML token found: %v", token)
	}
}

func stateProcInst(dev *Device, token xml.Token, pos xmlPos) (xmlState, error) {
	switch v := token.(type) {
	case xml.StartElement:
		return deviceMessagesState(dev, token, pos)

	case xml.EndElement:
		return xmlStateErr, errors.Errorf("found unexpected XML end element while scanning for XML header: %s", v.Name)

	case xml.CharData, xml.Comment, xml.Directive: // ignore
		log.Printf("ignore element %v", token)
		return xmlStateProcInst, nil

	case xml.ProcInst:
		log.Printf("ProcInst target=%s data=%s", v.Target, v.Inst)
		return xmlStateDeviceMessages, nil

	default:
		return xmlStateErr, errors.Errorf("unknown XML token found: %v", token)
	}
}

func stateEnd(dev *Device, token xml.Token, pos xmlPos) (xmlState, error) {
	switch token.(type) {
	case xml.CharData, xml.Comment, xml.Directive: // ignore
		log.Printf("ignore element %v", token)
		return xmlStateEnd, nil

	default:
		return xmlStateErr, errors.Errorf("unexpected XML token found after end: %v", token)
	}
}

func closeTagDecoder(expected xml.Name, next xmlState) stateFn {
	return func(dev *Device, token xml.Token, pos xmlPos) (xmlState, error) {
		switch v := token.(type) {
		case xml.EndElement:
			if v.Name != expected {
				return xmlStateErr, errors.Errorf("unexpected closing tag:%v found. Expected:%v", v.Name, expected)
			}
			return next, nil

		case xml.StartElement:
			return xmlStateErr, errors.Errorf("unexpected nesteg tag:%v found. Expected closing tag:%v", v.Name, expected)

		case xml.CharData, xml.Comment, xml.Directive, xml.ProcInst: // ignore
			return xmlStateErr, errors.Errorf("unexpected XML %v found while scanning for closing tag: %s", v, expected)

		default:
			return xmlStateErr, errors.Errorf("unknown XML token found: %v", token)
		}
	}
}

func tagDecoder(expected xml.Name, attribs map[xml.Name]attrExtractor, self xmlState, next xmlState) stateFn {
	return func(dev *Device, token xml.Token, pos xmlPos) (xmlState, error) {
		switch v := token.(type) {
		case xml.StartElement:
			if v.Name != expected {
				return xmlStateErr, errors.Errorf("unexpected tag:%v found. Expected:%v", v.Name, expected)
			}
			for _, attr := range v.Attr {
				fn, found := attribs[attr.Name]
				if !found {
					return xmlStateErr, errors.Errorf("attribute:%v not expected for tag:%v", attr.Name, v.Name)
				}
				if err := fn(attr.Value, dev, pos); err != nil {
					return xmlStateErr, errors.Wrapf(err, "error parsing attribute '%v'", attr.Name)
				}
			}
			return next, nil

		case xml.EndElement, xml.ProcInst:
			return xmlStateErr, errors.Errorf("unexpected XML %v found while scanning for start element: %s", v, expected)

		case xml.CharData, xml.Comment, xml.Directive: // ignore
			return self, nil

		default:
			return xmlStateErr, errors.Errorf("unknown XML token found: %v", token)
		}
	}
}

func (dev *Device) load() error {
	fHandle, err := os.Open(dev.XMLPath)
	if err != nil {
		return err
	}
	defer fHandle.Close()
	lineReader := util.NewLineReader(fHandle)
	decoder := xml.NewDecoder(lineReader)
	// TODO: Config flag
	decoder.Strict = true
	// Support custom charset inside XML.
	decoder.CharsetReader = charset.NewReaderLabel

	state := xmlStateProcInst
	pos := xmlPos{
		path: dev.XMLPath,
	}
	for {
		// Save pos before reading a token otherwise it points to the end
		// of a tag.
		pos.line, pos.col = lineReader.Position(uint64(decoder.InputOffset()))
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrapf(err, "error reading at %s", pos)
		}
		fn, exists := xmlStates[state]
		if !exists {
			return errors.Errorf("internal error: unknown state %v", state)
		}
		if state, err = fn(dev, token, pos); err != nil {
			return errors.Wrapf(err, "error decoding at %s", pos)
		}
	}
	if state != xmlStateEnd {
		return errors.Errorf("error decoding at %s: Unexpected file EOF", pos)
	}
	return nil
}
