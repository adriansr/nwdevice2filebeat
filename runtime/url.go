//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"net/url"
	"path"

	"github.com/pkg/errors"
	"golang.org/x/net/publicsuffix"

	"github.com/adriansr/nwdevice2filebeat/parser"
)

type urlExtract parser.URLExtract

var errUrlFieldNotFound = errors.New("source field for URL function not set")

func (ue urlExtract) Run(ctx *Context) error {
	value, err := ctx.Fields.Get(ue.Source)
	if err != nil {
		return errUrlFieldNotFound
	}
	result, err := ue.Extract(value)
	ctx.Fields.Put(ue.Target, result)
	return err
}

func (ue urlExtract) Extract(urlAsStr string) (result string, err error) {
	if urlAsStr == "" {
		return
	}
	url, err := url.Parse(urlAsStr)
	if err != nil {
		return result, err
	}

	var fakeScheme bool
	if len(url.Hostname()) == 0 && len(url.Path) != 0 && len(url.Scheme) == 0 {
		// A non-URL in the form "www.example.com" is understood as a relative
		// path by the url package. Need to compensate.
		if url, err = url.Parse("http://" + urlAsStr); err != nil {
			return result, err
		}
		fakeScheme = true
	}

	switch ue.Component {
	case parser.URLComponentDomain:
		if result, err = publicsuffix.EffectiveTLDPlusOne(url.Hostname()); err != nil {
			// This will still result in err being returned, which will be added
			// to event errors.
			result = url.Hostname()
		}
	case parser.URLComponentExt:
		result = path.Ext(url.Path)
	case parser.URLComponentFqdn:
		result = url.Hostname()
	case parser.URLComponentPage:
		// Page is referred to as "file name". Assuming it means
		//the last path component.
		_, file := path.Split(url.Path)
		result = file
	case parser.URLComponentPath:
		result = url.Path
	case parser.URLComponentPort:
		if result = url.Port(); result == "" && !fakeScheme {
			switch url.Scheme {
			case "http":
				result = "80"
			case "https":
				result = "443"
			default:
				err = errors.Errorf("in URL($PORT,...) no default port known for scheme '%s'", url.Scheme)
			}
		}
	case parser.URLComponentQuery:
		result = url.RawQuery
	case parser.URLComponentRoot:
		url.Path = "/" // Or empty?
		url.RawQuery = ""
		url.Fragment = ""
		result = url.String()
	}
	return
}
