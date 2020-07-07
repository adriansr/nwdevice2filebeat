//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package javascript

import (
	"fmt"
	"log"

	"github.com/joeshaw/multierror"
	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/parser"
)

// This needs to be disabled until proper dependency analysis is implemented.
// These two action chains are not equivalent:
//
// field_a: const1
// field_b: field_c
// field_c: const2
//
//
// set{
//   field_a: const1
//   field_c: const2
// }
// field_b: field_c
const promoteSetConstant = false

var preprocessors = parser.PostprocessGroup{
	Title: "javascript transforms",
	Actions: []parser.Action{
		{
			Name: "check calls to unknown functions",
			Run:  checkFunctionCalls,
		},
		{
			Name: "adjust overlapping payload capture",
			Run:  adjustOverlappingPayload,
		},
		{
			Name: "extract msg_id1",
			Run:  extractMsgID1,
		},
		{
			// Needs to run before adjustFieldNames so that SetField targets
			// don't have the prefix added to them.
			Name: "promote constant assignments",
			Run:  promoteConstantSetField,
		},
		{
			// Needs to run before adjustFieldNames so that SetField targets
			// don't have the prefix added to them.
			Name: "promote constant assignments",
			Run:  promoteConstantSetField,
		},
		{
			Name: "translate field constant assignment operations",
			Run:  translateConstantField,
		},
		{
			Name: "translate field copy operations",
			Run:  translateCopyField,
		},
		{
			Name: "forbid SetField",
			Run:  failIfSetFieldFound,
		},
		{
			Name: "adjust field names",
			Run:  adjustFieldNames,
		},
		/*{
			Name: "set @timestamp",
			Run:  setTimestamp,
		},*/

		// Some MESSAGE parsers don't capture anything. That's an error for
		// dissect so let's add an empty capture at the end.
		{
			Name: "Fix non-capturing dissects",
			Run:  fixNonCapturingDissects,
		},

		// From here down root node belongs to JS
		{
			Name: "prepare file structure",
			Run:  adjustTree,
		},
		{
			Name: "remove duplicates",
			Run:  removeDuplicateNodes,
		},
		{
			Name: "extract variables",
			Run:  extractVariables,
		},
	},
}

type MainProcessor struct {
	inner []parser.Operation
}

func (MainProcessor) Hashable() string {
	return ""
}

func (p MainProcessor) Children() []parser.Operation {
	return p.inner
}

type File struct {
	Nodes []parser.Operation
}

func (p File) Children() []parser.Operation {
	return p.Nodes
}

func (p File) WithVars(vars []parser.Operation) File {
	p.Nodes = append(p.Nodes, vars...)
	return p
}

func (File) Hashable() string {
	return ""
}

type RawJS string

func (p RawJS) String() string {
	return string(p)
}

func (RawJS) Hashable() string {
	return ""
}

func (p RawJS) Children() []parser.Operation {
	return nil
}

func adjustTree(p *parser.Parser) (err error) {
	var file File
	file.Nodes = append(file.Nodes,
		RawJS(header),
		MainProcessor{inner: []parser.Operation{p.Root}})
	for _, vm := range p.ValueMaps {
		file.Nodes = append(file.Nodes, vm)
	}
	p.Root = file
	return nil
}

func adjustFieldNames(p *parser.Parser) (err error) {
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		switch v := node.(type) {
		case parser.Match:
			if v.Input != "message" {
				v.Input = "nwparser." + v.Input
				return parser.WalkReplace, v
			}
			// TODO: Why prefix with nwparser in all these?
		case parser.Call:
			v.Target = "nwparser." + v.Target
			return parser.WalkReplace, v
		case parser.ValueMapCall:
			v.Target = "nwparser." + v.Target
			return parser.WalkReplace, v
		case parser.SetField:
			v.Target = "nwparser." + v.Target
			return parser.WalkReplace, v
		}
		return parser.WalkContinue, nil
	})
	return err
}

func adjustOverlappingPayload(p *parser.Parser) (err error) {
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		if match, ok := node.(parser.Match); ok && match.PayloadField != "" {
			var pos int
			var elem parser.Value
			found := false
			for pos, elem = range match.Pattern {
				if field, ok := elem.(parser.Field); ok && field.Name == match.PayloadField {
					found = true
					break
				}
			}
			if !found {
				err = errors.New("payload field not found")
				return parser.WalkCancel, nil
			}
			call := parser.Call{
				SourceContext: match.SourceContext,
				Function:      "STRCAT",
				Target:        "payload",
				Args:          match.Pattern[pos:],
			}
			match.OnSuccess = append(match.OnSuccess, call)
			return parser.WalkReplace, match
		}
		return parser.WalkContinue, nil
	})
	return err
}

var supportedJSFunctions = map[string]struct{}{
	"STRCAT": {},
	"SYSVAL": {},
	"HDR":    {},
	"DIRCHK": {},
	"DUR":    {},
	"URL":    {},
	"CALC":   {},
	"RMQ":    {},
	"UTC":    {},
}

func checkFunctionCalls(p *parser.Parser) (err error) {
	var errs multierror.Errors
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		if call, ok := node.(parser.Call); ok {
			if _, found := supportedJSFunctions[call.Function]; !found {
				errs = append(errs, errors.Errorf("at %s: found call to unsupported function '%s'", call.Source(), call.Function))
			}
		}
		return parser.WalkContinue, nil
	})
	return errs.Err()
}

func setTimestamp(p *parser.Parser) (err error) {
	timeFields := map[string]int{}
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		if datetime, ok := node.(parser.DateTime); ok {
			target := datetime.Target
			if datetime.Target == "" {
				err = errors.Errorf("at %s: no target for EVNTTIME", datetime.Source())
			}
			timeFields[target] += 1
		}
		return parser.WalkContinue, nil
	})
	if err != nil {
		return err
	}
	var selectedField string
	for _, field := range []string{"event_time", "eventtime", "recorded_time", "starttime"} {
		if timeFields[field] > 0 {
			selectedField = field
			break
		}
	}
	if selectedField == "" && len(timeFields) == 1 {
		for k := range timeFields {
			selectedField = k
		}
	}
	if selectedField != "" {
		rootChain := p.Root.(parser.Chain)
		rootChain.Nodes = append(rootChain.Nodes, parser.SetField{
			Target: "@timestamp",
			Value:  []parser.Operation{parser.Field{Name: selectedField}},
		})
		p.Root = rootChain
	} else {
		log.Printf("WARN: can't set @timestamp. Fields set by EVNTTIME: %+v", timeFields)
	}
	return nil
}

type VariableReference struct {
	Name string
}

type Variable struct {
	Name  string
	Value [1]parser.Operation
}

func (p Variable) Hashable() string {
	return "Variable{" + p.Name + "}"
}

func (v Variable) Children() []parser.Operation {
	return v.Value[:]
}

func (v VariableReference) Children() []parser.Operation {
	return nil
}

func (p VariableReference) Hashable() string {
	return "VarRef{" + p.Name + "}"
}

type nameGenerator struct {
	prefixes map[string]int
}

func (v *nameGenerator) New(prefix string) string {
	if v.prefixes == nil {
		v.prefixes = make(map[string]int)
	}
	v.prefixes[prefix] += 1
	return fmt.Sprintf("%s%d", prefix, v.prefixes[prefix])
}

func extractVariables(p *parser.Parser) (err error) {
	if !p.Config.Opt.GlobalEntities {
		return nil
	}
	file, ok := p.Root.(File)
	if !ok {
		return errors.New("operations tree is not a File")
	}
	var vars []parser.Operation
	var gen nameGenerator
	p.WalkPostOrder(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		var name string
		switch v := node.(type) {
		case parser.Chain:
			name = gen.New("chain")
		case parser.Match:
			switch v.Input {
			case "message":
				name = gen.New("hdr")
			case "payload":
				if !p.Config.Opt.StripMessageID1 {
					name = gen.New("msg")
				}
			default:
				name = gen.New("part")
			}
		case parser.LinearSelect:
			name = gen.New("select")
		case parser.AllMatch:
			name = gen.New("all")
		case MsgID1Wrapper:
			name = gen.New("msg")
		}
		if name == "" {
			return parser.WalkContinue, nil
		}
		vars = append(vars, Variable{
			Name:  name,
			Value: [1]parser.Operation{node},
		})
		return parser.WalkReplace, VariableReference{Name: name}
	})
	p.Root = file.WithVars(vars)
	return nil
}

func removeDuplicateNodes(p *parser.Parser) (err error) {
	if !p.Config.Opt.DetectDuplicates {
		return
	}
	var gen nameGenerator
	for pass := 1; ; pass++ {
		log.Printf("Removing duplicates pass %d", pass)
		in, out, err := doRemoveDuplicateNodes(p, &gen)
		if err != nil {
			return err
		}
		if in == out {
			log.Printf("Deduplication done. %d unique nodes left.", out)
			break
		}
		log.Printf("Duplicated from %d -> %d nodes", in, out)
	}
	return nil
}

func canBeDeduplicated(node parser.Operation) bool {
	switch node.(type) {
	case Variable, VariableReference, MainProcessor, RawJS, File:
		return false
	}
	return true
}

func doRemoveDuplicateNodes(p *parser.Parser, gen *nameGenerator) (inCount, outCount int, err error) {
	file, ok := p.Root.(File)
	if !ok {
		return 0, 0, errors.New("operations tree is not a File")
	}
	var vars []parser.Operation

	defer func() {
		p.Root = file.WithVars(vars)
	}()

	total := 0
	seen := make(map[string][]parser.Operation)
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		if canBeDeduplicated(node) {
			total++
			hash := node.Hashable()
			if hash == "" {
				err = errors.Errorf("found null hashable of type %T", node)
			}
			seen[hash] = append(seen[hash], node)
		}
		return parser.WalkContinue, nil
	})
	if len(seen) == total {
		return total, total, err
	}
	for k, v := range seen {
		if len(v) < 2 {
			delete(seen, k)
		} else {
			//log.Printf(" Will deduplicate (%d) : %s", len(v), k)
			seen[k] = nil
		}
	}
	p.WalkPostOrder(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		if canBeDeduplicated(node) {
			hash := node.Hashable()
			if ref, ok := seen[hash]; ok {
				var repl parser.Operation
				if ref == nil {
					name := gen.New("dup")
					vars = append(vars, Variable{
						Name:  name,
						Value: [1]parser.Operation{node},
					})
					repl = VariableReference{Name: name}
					seen[hash] = []parser.Operation{repl}
				} else {
					repl = ref[0]
				}
				return parser.WalkReplace, repl
			}
		}
		return parser.WalkContinue, nil
	})
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		if canBeDeduplicated(node) {
			outCount++
			hash := node.Hashable()
			if _, seen := seen[hash]; seen {
				// This detects nodes whose memory layout prevents child nodes
				// from being replaced.
				log.Printf("WARN: Should've been removed: %s", hash)
			}
		}
		return parser.WalkContinue, nil
	})

	return total, outCount, nil
}

func fixNonCapturingDissects(p *parser.Parser) error {
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		if match, ok := node.(parser.Match); ok {
			if len(match.Pattern) != 1 {
				return parser.WalkContinue, nil
			}
			if _, ok := match.Pattern[0].(parser.Constant); !ok {
				return parser.WalkContinue, nil
			}
			match.Pattern = append(match.Pattern, parser.Field{})
			return parser.WalkReplace, match
		}
		return parser.WalkContinue, nil
	})
	return nil
}

type SetProcessor map[string]string

func (s SetProcessor) Hashable() string {
	return fmt.Sprintf("SetP{%v}", s)
}

func (s SetProcessor) Children() []parser.Operation {
	return nil
}

func promoteConstantSetFieldList(list []parser.Operation) (newList []parser.Operation, err error) {
	processor := SetProcessor{}
	var delete []int
	for idx, op := range list {
		if set, isSet := op.(parser.SetField); isSet {
			if ct, isCt := set.Value[0].(parser.Constant); isCt {
				if prev, exists := processor[set.Target]; exists && prev != ct.Value() {
					log.Printf("at %s: field '%s' is set more than once (values '%s' and '%s')",
						set.Source(), set.Target, prev, ct.Value())
				}
				processor[set.Target] = ct.Value()
				delete = append(delete, idx)
			}
		}
	}
	if len(processor) > 0 {
		newList = append(append(newList, processor),
			parser.OpList(list).Remove(delete)...)
	}
	return newList, nil
}

func promoteConstantSetField(p *parser.Parser) (err error) {
	// WARNING: This is dangerous because it messes with the order in which
	//          functions are executed inside a on_success chain. Should be
	//          updated to only extract fields whose value is not accessed
	//          before being set (They could be the result of a capture, copied
	//          to another field and then set to a different value).
	if !promoteSetConstant {
		return nil
	}

	// This usually leads to a smaller JS even though now we cannot deduplicate
	// constant fields set as hardly two messages will set the exact same fields.
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		var list []parser.Operation
		switch v := node.(type) {
		case parser.Match:
			list, err = promoteConstantSetFieldList(v.OnSuccess)
			if err != nil {
				return parser.WalkCancel, nil
			}
			if list != nil {
				v.OnSuccess = list
				return parser.WalkReplace, v
			}
		case parser.AllMatch:
			changed := false
			list, err = promoteConstantSetFieldList(v.OnSuccess())
			if err != nil {
				return parser.WalkCancel, nil
			}
			if list != nil {
				v = v.WithOnSuccess(list)
				changed = true
			}
			list, err = promoteConstantSetFieldList(v.OnFailure())
			if err != nil {
				return parser.WalkCancel, nil
			}
			if list != nil {
				v = v.WithOnFailure(list)
				changed = true
			}
			if changed {
				return parser.WalkReplace, v
			}
		}
		return parser.WalkContinue, nil
	})
	return err
}

type SetField [2]string

func (s SetField) Hashable() string {
	return fmt.Sprintf("SetF{dst=%s,src=%s}", s[0], s[1])
}

func (s SetField) Children() []parser.Operation {
	return nil
}

type SetConstant [2]string

func (s SetConstant) Hashable() string {
	return fmt.Sprintf("SetC{dst=%s,src=%s}", s[0], s[1])
}

func (s SetConstant) Children() []parser.Operation {
	return nil
}

func promoteCopyFieldInList(list []parser.Operation) (changed bool) {
	for idx, op := range list {
		if set, isSet := op.(parser.SetField); isSet {
			if field, isField := set.Value[0].(parser.Field); isField {
				list[idx] = SetField{
					set.Target,
					field.Name,
				}
				changed = true
			}
		}
	}
	return changed
}

func promoteConstantFieldInList(list []parser.Operation) (changed bool) {
	for idx, op := range list {
		if set, isSet := op.(parser.SetField); isSet {
			if ct, isCt := set.Value[0].(parser.Constant); isCt {
				list[idx] = SetConstant{
					set.Target,
					ct.Value(),
				}
				changed = true
			}
		}
	}
	return changed
}

func doTranslateSetField(p *parser.Parser, translator func([]parser.Operation) bool) error {
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		switch v := node.(type) {
		case parser.Match:
			if translator(v.OnSuccess) {
				return parser.WalkReplace, v
			}
		case parser.AllMatch:
			changed := translator(v.OnSuccess())
			changed = translator(v.OnFailure()) || changed
			if changed {
				return parser.WalkReplace, v
			}
		}
		return parser.WalkContinue, nil
	})
	return nil
}

func translateCopyField(p *parser.Parser) error {
	return doTranslateSetField(p, promoteCopyFieldInList)
}

func translateConstantField(p *parser.Parser) error {
	return doTranslateSetField(p, promoteConstantFieldInList)
}

func failIfSetFieldFound(p *parser.Parser) (err error) {
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		switch v := node.(type) {
		case parser.SetField:
			err = errors.Errorf("parser.SetField found where it shouldn't: %s", v.Hashable())
			return parser.WalkCancel, nil
		}
		return parser.WalkContinue, nil
	})
	return err
}

type MsgID1Wrapper struct {
	msgID1  string
	wrapped []parser.Operation
}

func (m MsgID1Wrapper) Hashable() string {
	return fmt.Sprintf("ID1{%s, %s}", m.msgID1, m.wrapped[0].Hashable())
}

func (m MsgID1Wrapper) Children() []parser.Operation {
	return m.wrapped
}

func findMsgID1(actions []parser.Operation) (found bool, msgID1 string, newList []parser.Operation) {
	for idx, act := range actions {
		if setOp, ok := act.(parser.SetField); ok && setOp.Target == "msg_id1" {
			if ct, ok := setOp.Value[0].(parser.Constant); ok {
				return true, ct.Value(), append(append([]parser.Operation{}, actions[:idx]...), actions[idx+1:]...)
			}
		}
	}
	return
}

func extractMsgID1(p *parser.Parser) (err error) {
	if false {
		return nil
	}
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		switch v := node.(type) {
		case parser.AllMatch:
			if found, id1, list := findMsgID1(v.OnSuccess()); found {
				cp := v.WithOnSuccess(list)
				return parser.WalkReplace, MsgID1Wrapper{
					msgID1:  id1,
					wrapped: []parser.Operation{cp},
				}
			}
		case parser.Match:
			if found, id1, list := findMsgID1(v.OnSuccess); found {
				cp := v
				cp.OnSuccess = list
				return parser.WalkReplace, MsgID1Wrapper{
					msgID1:  id1,
					wrapped: []parser.Operation{cp},
				}
			}
		}
		return parser.WalkContinue, nil
	})
	return err
}
