//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package util

import (
	"fmt"
	"log"
)

type Warning struct {
	Pos  XMLPos
	Text string
}

type Warnings struct {
	Message []Warning
	Total   int
	limit   int
}

func NewWarnings(limit int) Warnings {
	return Warnings{
		limit: limit,
	}
}

func (w *Warnings) Add(pos XMLPos, msg string) {
	if w.accept() {
		w.Message = append(w.Message, Warning{Pos: pos, Text: msg})
	}
}

func (w *Warnings) accept() bool {
	if w == nil {
		return false
	}
	w.Total++
	if w.Total > w.limit {
		return false
	}
	return true
}

func (w *Warnings) Addf(pos XMLPos, format string, args ...interface{}) {
	if w.accept() {
		w.Message = append(w.Message, Warning{Pos: pos, Text: fmt.Sprintf(format, args...)})
	}
}

func (w *Warnings) Print(label string) bool {
	if w == nil || w.Total == 0 {
		return false
	}
	log.Printf("Found %d warnings while %s:", w.Total, label)
	for idx, msg := range w.Message {
		log.Printf("  [%d] at %s: %s", idx+1, msg.Pos, msg.Text)
	}
	if len(w.Message) < w.Total {
		log.Printf("  [...] and %d more.", w.Total-len(w.Message))
	}
	return true
}

func (w *Warnings) Clear() {
	if w == nil {
		return
	}
	w.Message = nil
	w.Total = 0
}
