// encoding exposes functionality to convert between an arbitray maps and
// native Go structs in service of the Vanflow protocol.
//
// The Vanflow protocol uses the amqp-value section to represent record types
// in map structures. Typically go amqp clients perform the marshaling to and
// from byte representation to some arbitrary map types, usually map[any]any.
// This package provides mapping to and from that map type to Go types.
//
// All vanflow record types have an associated record type codepoint. Types
// need to be registered with that codepoint using MustRegisterRecord before
// they can be used.
//
// When decoding to a struct, encoding will look for `vflow` struct fields to
// establish a mapping between vanflow record attribute codepoints and struct
// fields. If a record attribute is required as part of the record, appending
// ",required" to the vflow tag value will enforce that the field is set.
//
// For Example:
//
//	type ExampleRecord struct {
//	  ExampleID int64 `vflow:"99,required"`
//	  ExampleVal string `vflow:"100"`
//	}
//
// The associated record attribute set would look like this:
//
//	map[any]any{
//	  uint32(0): ExampleRecordCodepoint,
//	  uint32(99): int64(1),
//	  uint32(100): "test",
//	}
package encoding

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

var (
	mu        sync.RWMutex
	encodings = map[reflect.Type]*typeEncoding{}
	decodings = map[uint32]*typeEncoding{}
)

const (
	vflowTag     = "vflow"
	typeOfRecord = uint32(0)
)

type typeEncoding struct {
	t         reflect.Type
	codepoint uint32
	fields    []field
}

// MustRegisterRecord registers a record type for Encoding/Decoding panics if
// the type is not compatible, or if the codepoint or type has already been
// registered.
func MustRegisterRecord(codepoint uint32, record interface{}) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := decodings[codepoint]; ok {
		panic(fmt.Sprintf("cannot register record type %T using codepoint %d: already in use", record, codepoint))
	}

	encoding := newEncodingForType(codepoint, record)
	if existing, ok := encodings[encoding.t]; ok {
		panic(fmt.Sprintf("cannot register same type more than once. type %T already registered with code %d", record, existing.codepoint))
	}
	encodings[encoding.t] = encoding
	decodings[codepoint] = encoding
}

func newEncodingForType(codepoint uint32, record interface{}) *typeEncoding {
	recordType := reflect.Indirect(reflect.ValueOf(record)).Type()
	encoding := typeEncoding{codepoint: codepoint, t: recordType}

	if recordType.Kind() != reflect.Struct {
		panic(fmt.Sprintf("unsupported encoder type %v. Expects struct type", recordType))
	}

	fields := fieldsForType(recordType, nil)
	codepoints := map[uint32]field{}
	for _, field := range fields {
		if existing, ok := codepoints[field.Codepoint]; ok {
			panic(fmt.Sprintf(
				"struct field %s repeats vflow tag \"%d\" also used by %s",
				field.Name, field.Codepoint, existing.Name))
		}
		codepoints[field.Codepoint] = field
	}
	encoding.fields = fields
	return &encoding
}

type field struct {
	Name      string
	Codepoint uint32
	Identity  bool

	index   []int
	encoder fieldEncoder
	decoder fieldDecoder
}

func parseFieldOpts(f reflect.StructField) (field, bool) {
	opts := field{
		Name: f.Name,
	}

	tag := f.Tag.Get(vflowTag)
	if tag == "" {
		return opts, false
	}
	sCode, sOpt, _ := strings.Cut(tag, ",")
	code, err := strconv.ParseUint(sCode, 10, 32)
	if err != nil {
		panic(fmt.Sprintf("vflow struct tag parse error for field %s: %s", f.Name, err))
	}
	opts.Codepoint = uint32(code)
	switch suffix := sOpt; suffix {
	case "required":
		opts.Identity = true
	case "":
	default:
		panic(fmt.Sprintf("vflow struct tag parse error: unexpected option %s", suffix))
	}
	return opts, true
}

func (e typeEncoding) encode(root reflect.Value) (RecordAttributeSet, error) {
	attributeSet := make(RecordAttributeSet)
	var v reflect.Value
	for _, field := range e.fields {
		v = root
		for _, idx := range field.index {
			v = v.Field(idx)
		}
		out, err := field.encoder.encode(v)
		if err != nil {
			if errors.Is(err, ErrAttributeNotSet) {
				if field.Identity {
					return attributeSet, fmt.Errorf("missing or empty required field %s: %s", field.Name, err)
				}
				continue // ignore if not required
			}
			return attributeSet, fmt.Errorf("error encoding field %q: %w", field.Name, err)
		}
		if out != nil {
			attributeSet[field.Codepoint] = out
		}
	}
	return attributeSet, nil
}

func (e typeEncoding) decode(attrs RecordAttributeSet, root reflect.Value) error {
	var v reflect.Value
	for _, field := range e.fields {
		obj, ok := attrs[field.Codepoint]
		if !ok {
			if field.Identity {
				return fmt.Errorf("record attribute set missing required field %q", field.Name)
			}
			continue
		}
		v = root.Elem()
		for _, idx := range field.index {
			v = v.Field(idx)
		}
		err := field.decoder.decode(obj, v)
		if err != nil {
			return fmt.Errorf("error decoding field %q: %w", field.Name, err)
		}
	}
	return nil
}

func fieldsForType(t reflect.Type, index []int) []field {
	var fields []field
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		next := make([]int, len(index)+1)
		copy(next, index)
		next[len(next)-1] = i
		// Include fields from embedded structs
		if f.Anonymous {
			if f.Type.Kind() == reflect.Struct {
				fields = append(fields, fieldsForType(f.Type, next)...)
			}
			continue
		} else if !f.IsExported() {
			// ignore unexported fields
			continue
		}

		fieldOpts, ok := parseFieldOpts(f)
		if !ok {
			continue
		}
		fieldOpts.index = next
		encoder, err := getFieldEncoder(f.Type)
		if err != nil {
			panic(fmt.Sprintf("invalid vflow field encoder for %q: %s", f.Name, err))
		}
		fieldOpts.encoder = encoder
		decoder, err := getFieldDecoder(f.Type)
		if err != nil {
			panic(fmt.Sprintf("invalid vflow field decoder for %q: %s", f.Name, err))
		}
		fieldOpts.decoder = decoder
		fields = append(fields, fieldOpts)
	}
	return fields
}
