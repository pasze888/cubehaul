package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/itchyny/gojq"
)

// Selector describes what the --json flag asked for.
type Selector struct {
	On     bool     // --json was passed
	All    bool     // bare --json: every declared field
	Fields []string // --json=a,b: exactly these fields, in this order
}

// ParseSelector turns the raw --json value into a Selector.
//
// An empty value means no JSON at all ("--json=" lands here too). cobra stores
// "all" for a bare "--json", because the flag carries a NoOptDefVal; anything
// else is a comma-separated field list.
func ParseSelector(raw string) Selector {
	raw = strings.TrimSpace(raw)
	switch raw {
	case "":
		return Selector{}
	case "all":
		return Selector{On: true, All: true}
	}
	fields := make([]string, 0, 8)
	for _, name := range strings.Split(raw, ",") {
		if name = strings.TrimSpace(name); name != "" {
			fields = append(fields, name)
		}
	}
	return Selector{On: true, Fields: fields}
}

// Format carries the output flags shared by every command.
type Format struct {
	JSON Selector
	JQ   string
}

// NewFormat builds the format from the raw --json and --jq values. A jq
// expression implies JSON output, so --jq works on its own.
func NewFormat(jsonValue, jq string) Format {
	f := Format{JSON: ParseSelector(jsonValue), JQ: strings.TrimSpace(jq)}
	if f.JQ != "" && !f.JSON.On {
		f.JSON = Selector{On: true, All: true}
	}
	return f
}

// selected returns the field names to emit for a resource whose contract is
// declared, rejecting unknown names the way gh lists the JSON fields it accepts.
func (s Selector) selected(declared []string) ([]string, error) {
	switch {
	case !s.On:
		return nil, nil
	case s.All:
		return declared, nil
	}
	for _, name := range s.Fields {
		if !slices.Contains(declared, name) {
			return nil, fmt.Errorf("unknown --json field %q\nvalid fields: %s", name, strings.Join(declared, ", "))
		}
	}
	return s.Fields, nil
}

// record projects one value onto the selected fields. Each field keeps its own
// JSON encoding, so nested objects and arrays pass through untouched.
func (f Format) record(v any, declared []string) ([]byte, error) {
	names, err := f.JSON.selected(declared)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	byName := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &byName); err != nil {
		return nil, fmt.Errorf("internal error: %T is not a JSON object: %w", v, err)
	}

	var b bytes.Buffer
	b.WriteByte('{')
	written := 0
	for _, name := range names {
		value, ok := byName[name]
		if !ok { // a field this value leaves out (omitempty)
			continue
		}
		if written > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(name)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(value)
		written++
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// emitList writes a JSON array of projected records.
func emitList[T any](f Format, items []T, declared []string) error {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, item := range items {
		rec, err := f.record(item, declared)
		if err != nil {
			return err
		}
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(rec)
	}
	b.WriteByte(']')
	return f.emit(b.Bytes())
}

// emitOne writes a single projected record.
func emitOne(f Format, v any, declared []string) error {
	rec, err := f.record(v, declared)
	if err != nil {
		return err
	}
	return f.emit(rec)
}

// emit finishes a JSON document: filtered by --jq when given, otherwise
// pretty-printed as is.
func (f Format) emit(doc []byte) error {
	if f.JQ != "" {
		return runJQ(doc, f.JQ)
	}
	return writeDocument(doc)
}

// Document writes an arbitrary JSON document, used by "api" where the response
// shape comes from the remote API rather than from this tool.
func (f Format) Document(doc []byte) error { return f.emit(doc) }

// JSON writes v as indented JSON, for results that take no field selection.
func JSON(v any) error {
	doc, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeDocument(doc)
}

// writeDocument pretty-prints one JSON document on stdout.
func writeDocument(doc []byte) error {
	var b bytes.Buffer
	if err := json.Indent(&b, doc, "", "  "); err != nil {
		return fmt.Errorf("internal error: %w", err)
	}
	b.WriteByte('\n')
	_, err := os.Stdout.Write(b.Bytes())
	return err
}

// runJQ evaluates expr against doc. gojq implements jq's semantics in pure Go,
// so no jq binary is needed (the same choice gh makes). Strings print raw and
// everything else as JSON, one result per line.
func runJQ(doc []byte, expr string) error {
	query, err := gojq.Parse(expr)
	if err != nil {
		return fmt.Errorf("invalid --jq expression: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber() // keep large download counts exact
	var input any
	if err := dec.Decode(&input); err != nil {
		return fmt.Errorf("internal error: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	iter := query.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if err, ok := v.(error); ok {
			return fmt.Errorf("--jq: %w", err)
		}
		if s, ok := v.(string); ok {
			fmt.Println(s)
			continue
		}
		if err := enc.Encode(v); err != nil {
			return err
		}
	}
}
