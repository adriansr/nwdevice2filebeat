//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package model

import (
	"compress/gzip"
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
	XMLPath     string
	Description DeviceHeader
	Version     Version

	Headers    []*Header
	Messages   []*Message
	TagValMaps []*TagValMap
	ValueMaps  []*ValueMap
	VarTypes   []*VarType
	Regexs     []*RegX
	SumDatas   []*SumData
}

// New turns a new Device from the given directory path.
func NewDevice(path string, _ *util.Warnings) (Device, error) {
	files, err := listFilesByExtensions(path)
	if err != nil {
		return Device{}, err
	}
	log.Printf("Found files: %+v", files)

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
	if err = dev.load(); err != nil {
		return dev, err
	}
	return dev, nil
}

// New turns a new Device from the given path to an (optionally compressed) XML.
func NewDeviceFromXML(path string) (Device, error) {
	dev := Device{
		XMLPath: path,
	}
	if err := dev.load(); err != nil {
		return dev, err
	}
	return dev, nil
}

func (dev *Device) String() string {
	return fmt.Sprintf("device={%s, %s, xml:'%s'}", dev.Description.String(), dev.Version.String(), dev.XMLPath)
}

func (dev *Device) load() error {
	fHandle, err := os.Open(dev.XMLPath)
	if err != nil {
		return err
	}
	defer fHandle.Close()
	fileReader := io.ReadCloser(fHandle)
	if strings.HasSuffix(dev.XMLPath, ".gz") {
		if fileReader, err = gzip.NewReader(fHandle); err != nil {
			return errors.Wrapf(err, "failed reading file in gzip format")
		}
		defer fileReader.Close()
	}
	lineReader := util.NewLineReader(fileReader)
	decoder := xml.NewDecoder(lineReader)
	// TODO: Config flag
	decoder.Strict = true
	// Support custom charset inside XML.
	decoder.CharsetReader = charset.NewReaderLabel

	state := xmlStateProcInst
	pos := util.XMLPos{
		Path: dev.XMLPath,
	}

	numItems := 0
	for {
		// Save pos before reading a token otherwise it points to the end
		// of a tag.
		pos.Line, pos.Col = lineReader.Position(uint64(decoder.InputOffset()))
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrapf(err, "error reading at %s", pos)
		}
		fn, exists := xmlStates[state]
		if !exists {
			return errors.Errorf("at %s: internal error: unknown state %v", pos, state)
		}
		var item XMLElement
		item, state, err = fn(token, decoder)
		if err != nil {
			return errors.Wrapf(err, "error decoding at %s", pos)
		}
		if item != nil {
			numItems++
			item.SetPos(pos)
			if err = item.Apply(dev); err != nil {
				return errors.Wrapf(err, "error applying item at %s", pos)
			}
		}
	}
	if state != xmlStateEnd {
		return errors.Errorf("error decoding at %s: Unexpected EOF", pos)
	}
	log.Printf("loaded %d elements", numItems)
	return nil
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

type stateFn func(token xml.Token, decoder *xml.Decoder) (XMLElement, xmlState, error)

func name(n string) xml.Name {
	return xml.Name{
		Local: n,
	}
}

func displayName(x xml.Name) string {
	if x.Space == "" {
		return x.Local
	}
	return x.Space + ":" + x.Local
}

type xmlState int

const (
	xmlStateErr xmlState = iota
	xmlStateProcInst
	xmlStateDeviceMessages
	xmlStateBody
	xmlStateEnd
)

var xmlStates = map[xmlState]stateFn{
	xmlStateProcInst:       stateProcInst,
	xmlStateBody:           stateBody,
	xmlStateDeviceMessages: stateDeviceMessages,
	xmlStateEnd:            stateEnd,
}

type XMLElement interface {
	XMLDecodingError() error
	Pos() util.XMLPos
	SetPos(util.XMLPos)
	Apply(*Device) error
}

type XMLBaseElement struct {
	location    util.XMLPos
	UnknownAttr []xml.Attr `xml:",any,attr"`
	UnknownXML  []byte     `xml:",innerxml"`
}

func (e *XMLBaseElement) Pos() util.XMLPos {
	return e.location
}

func (e *XMLBaseElement) SetPos(p util.XMLPos) {
	e.location = p
}

func (e *XMLBaseElement) XMLDecodingError() error {
	hasAttr, hasXML := len(e.UnknownAttr) != 0, len(e.UnknownXML) != 0
	if !hasAttr && !hasXML {
		return nil
	}
	var sb strings.Builder
	if hasAttr {
		sb.WriteString("unknown attributes=")
		for idx, attr := range e.UnknownAttr {
			if idx > 0 {
				sb.WriteByte(',')
			} else {
				sb.WriteByte('[')
			}
			sb.WriteByte('"')
			sb.WriteString(displayName(attr.Name))
			sb.WriteByte('"')
		}
		sb.WriteByte(']')
	}
	if hasXML {
		if hasAttr {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("unexpected inner XML=\"%s\"", string(e.UnknownXML)))
	}
	return errors.New(sb.String())
}

type DeviceHeader struct {
	XMLBaseElement
	Name        string `xml:"name,attr"`
	DisplayName string `xml:"displayname,attr"`
	Group       string `xml:"group,attr"`
}

func (h DeviceHeader) String() string {
	return fmt.Sprintf("description={name:'%s', displayName:'%s', group:'%s'}",
		h.Name, h.DisplayName, h.Group)
}

func (h *DeviceHeader) Apply(dev *Device) error {
	dev.Description = *h
	return nil
}

type Header struct {
	XMLBaseElement
	ID1           string `xml:"id1,attr"`
	ID2           string `xml:"id2,attr"`
	EventCategory string `xml:"eventcategory,attr"`
	MissField     string `xml:"missField,attr"`
	TagVal        string `xml:"tagval,attr"`
	Functions     string `xml:"functions,attr"`
	Content       string `xml:"content,attr"`
	MessageID     string `xml:"messageid,attr"`
	Prioritize    string `xml:"prioritize,attr"`
	Devts         string `xml:"devts,attr"`
}

func (h *Header) Apply(dev *Device) error {
	dev.Headers = append(dev.Headers, h)
	return nil
}

type Version struct {
	XMLBaseElement
	XML      string `xml:"xml,attr"`
	Checksum string `xml:"checksum,attr"`
	Revision string `xml:"revision,attr"`
	Device   string `xml:"device,attr"`
	EnVision string `xml:"enVision,attr"`
}

func (v *Version) String() string {
	return fmt.Sprintf("version={xml:'%s', revision:'%s', device:'%s', checksum:'%s', enVision:'%s'",
		v.XML, v.Revision, v.Device, v.Checksum, v.EnVision)
}

func (v *Version) Apply(dev *Device) error {
	var unset util.XMLPos
	if dev.Version.Pos() != unset {
		return errors.Errorf("VERSION already set from %s", dev.Version.Pos())
	}
	dev.Version = *v
	return nil
}

type TagValMap struct {
	XMLBaseElement
	Delimiter         string `xml:"delimiter,attr"`
	ValueDelimiter    string `xml:"valuedelimiter,attr"`
	PairDelimiter     string `xml:"pairdelimiter,attr"`
	EscapeValueDelimt string `xml:"escapeValueDelim,attr"`
	Encapsulator      string `xml:"encapsulator,attr"`
}

func (tvm *TagValMap) Apply(dev *Device) error {
	dev.TagValMaps = append(dev.TagValMaps, tvm)
	return nil
}

type ValueMap struct {
	XMLBaseElement
	Name          string `xml:"name,attr"`
	Default       string `xml:"default,attr"`
	KeyValuePairs string `xml:"keyvaluepairs,attr"`
}

func (vm *ValueMap) Apply(dev *Device) error {
	dev.ValueMaps = append(dev.ValueMaps, vm)
	return nil
}

type Message struct {
	XMLBaseElement
	ID1           string `xml:"id1,attr"`
	ID2           string `xml:"id2,attr"`
	EventCategory string `xml:"eventcategory,attr"`
	MissField     string `xml:"missField,attr"`
	TagVal        string `xml:"tagval,attr"`
	Functions     string `xml:"functions,attr"`
	Content       string `xml:"content,attr"`
	MessageID     string `xml:"messageid,attr"`
	Level         string `xml:"level,attr"`
	Parse         string `xml:"parse,attr"`
	ParseDefValue string `xml:"parsedefvalue,attr"`
	TableID       string `xml:"tableid,attr"`
	Summary       string `xml:"summary,attr"`
}

func (m *Message) Apply(dev *Device) error {
	dev.Messages = append(dev.Messages, m)
	return nil
}

type VarType struct {
	XMLBaseElement
	Name       string `xml:"name,attr"`
	Regex      string `xml:"regex,attr"`
	IgnoreCase string `xml:"ignorecase,attr"`
}

func (vt *VarType) Apply(dev *Device) error {
	dev.VarTypes = append(dev.VarTypes, vt)
	return nil
}

type RegX struct {
	XMLBaseElement
	Name    string `xml:"name,attr"`
	Parms   string `xml:"parms,attr"`
	Default string `xml:"default,attr"`
	Expr    string `xml:"expr,attr"`
}

func (rx *RegX) Apply(dev *Device) error {
	dev.Regexs = append(dev.Regexs, rx)
	return nil
}

type SumData struct {
	XMLBaseElement
	Bucket string `xml:"bucket,attr"`
	Key    string `xml:"key,attr"`
	SubKey string `xml:"subkey,attr"`
	Fields string `xml:"fields,attr"`
}

func (sd *SumData) Apply(dev *Device) error {
	dev.SumDatas = append(dev.SumDatas, sd)
	return nil
}

var allowedBodyTags = map[xml.Name]func() XMLElement{
	name("HEADER"):    func() XMLElement { return new(Header) },
	name("VERSION"):   func() XMLElement { return new(Version) },
	name("TAGVALMAP"): func() XMLElement { return new(TagValMap) },
	name("VALUEMAP"):  func() XMLElement { return new(ValueMap) },
	name("MESSAGE"):   func() XMLElement { return new(Message) },
	name("VARTYPE"):   func() XMLElement { return new(VarType) },
	name("REGX"):      func() XMLElement { return new(RegX) },
	name("SUMDATA"):   func() XMLElement { return new(SumData) },
}

func stateBody(token xml.Token, decoder *xml.Decoder) (XMLElement, xmlState, error) {
	switch v := token.(type) {
	case xml.StartElement:
		alloc, ok := allowedBodyTags[v.Name]
		if !ok {
			return nil, xmlStateErr, errors.Errorf("unexpected XML tag found: %s", displayName(v.Name))
		}
		e := alloc()
		if err := decoder.DecodeElement(&e, &v); err != nil {
			return nil, xmlStateErr, errors.Wrapf(err, "error decoding tag %s", displayName(v.Name))
		}
		if err := e.XMLDecodingError(); err != nil {
			return nil, xmlStateErr, errors.Wrapf(err, "unexpected data decoding tag %s", displayName(v.Name))
		}
		return e, xmlStateBody, nil

	case xml.ProcInst:
		return nil, xmlStateErr, errors.Errorf("unexpected XML: ProcInst found while scanning for body")

	case xml.CharData, xml.Comment, xml.Directive: // ignore
		return nil, xmlStateBody, nil

	case xml.EndElement:
		if v.Name != name("DEVICEMESSAGES") {
			return nil, xmlStateErr, errors.Errorf("unexpected closing tag:%s", displayName(v.Name))
		}
		return nil, xmlStateEnd, nil

	default:
		return nil, xmlStateErr, errors.Errorf("unknown XML token found: %v", token)
	}
}

func stateDeviceMessages(token xml.Token, _ *xml.Decoder) (XMLElement, xmlState, error) {
	expected := name("DEVICEMESSAGES")
	switch v := token.(type) {
	case xml.StartElement:
		if v.Name != expected {
			return nil, xmlStateErr, errors.Errorf("unexpected tag:%v found. Expected:%v", v.Name, expected)
		}
		var dm DeviceHeader
		// Manual decoding :(
		// There is no way to decode this element using standard library without
		// decoding all the inner XML (messages, headers, etc.)
		for _, attr := range v.Attr {
			switch name := displayName(attr.Name); name {
			case "name":
				dm.Name = attr.Value
			case "displayname":
				dm.DisplayName = attr.Value
			case "group":
				dm.Group = attr.Value
			default:
				return nil, xmlStateErr, errors.Errorf("unexpected attribute in %s: %s", displayName(expected), name)
			}
		}
		return &dm, xmlStateBody, nil

	case xml.ProcInst:
		return nil, xmlStateErr, errors.Errorf("unexpected XML: ProcInst found while scanning for %s", displayName(expected))

	case xml.CharData, xml.Comment, xml.Directive: // ignore
		return nil, xmlStateDeviceMessages, nil

	case xml.EndElement:
		return nil, xmlStateErr, errors.Errorf("unexpected closing tag:%s", displayName(v.Name))

	default:
		return nil, xmlStateErr, errors.Errorf("unknown XML token found: %v", token)
	}
}

func stateProcInst(token xml.Token, decoder *xml.Decoder) (XMLElement, xmlState, error) {
	switch v := token.(type) {
	case xml.StartElement:
		return stateDeviceMessages(token, decoder)

	case xml.EndElement:
		return nil, xmlStateErr, errors.Errorf("found unexpected XML end element while scanning for XML header: %s", v.Name)

	case xml.CharData, xml.Comment, xml.Directive: // ignore
		return nil, xmlStateProcInst, nil

	case xml.ProcInst:
		return nil, xmlStateDeviceMessages, nil

	default:
		return nil, xmlStateErr, errors.Errorf("unknown XML token found: %v", token)
	}
}

func stateEnd(token xml.Token, _ *xml.Decoder) (XMLElement, xmlState, error) {
	switch token.(type) {
	case xml.CharData, xml.Comment, xml.Directive: // ignore
		return nil, xmlStateEnd, nil

	default:
		return nil, xmlStateErr, errors.Errorf("unexpected XML token found after end: %v", token)
	}
}
