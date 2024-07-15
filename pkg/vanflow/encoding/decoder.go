package encoding

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	recordAttributeDecoderType = reflect.TypeOf((*RecordAttributeDecoder)(nil)).Elem()
)

// RecordAttributeDecoder is implemented by record attribute types that can
// handle their own decoding.
type RecordAttributeDecoder interface {
	DecodeRecordAttribute(attr interface{}) error
}

// Decode a record from its record attribute set. Decode is only aware of
// record types registered with MustRegisterRecord. Input must be a map type
// containing uint32 keys (map[uint32]X or map[any]X)
func Decode(recordset RecordAttributeSet) (interface{}, error) {
	if recordset == nil {
		return nil, errors.New("decode error: cannot decode nil record attribute set")
	}
	recordTypeCode, ok := recordset[typeOfRecord]
	if !ok {
		return nil, errors.New("decode error: record type attribute not present")
	}

	codepoint, ok := recordTypeCode.(uint32)
	if !ok {
		return nil, fmt.Errorf("decode error: unexpected type for record type attribute \"%T\"", recordTypeCode)
	}

	mu.RLock()
	defer mu.RUnlock()
	encoding, ok := decodings[codepoint]
	if !ok {
		return nil, fmt.Errorf("decode error: unknown record type for %d", recordTypeCode)
	}
	recordV := reflect.New(encoding.t)
	if err := encoding.decode(recordset, recordV); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}
	return recordV.Elem().Interface(), nil
}

type fieldDecoder interface {
	decode(obj interface{}, v reflect.Value) error
}

func getFieldDecoder(t reflect.Type) (fieldDecoder, error) {
	if t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(recordAttributeDecoderType) {
		return recordAttrFieldDecoder{}, nil
	}
	if t.Implements(recordAttributeDecoderType) {
		return recordAttrFieldDecoder{}, nil
	}
	switch t.Kind() {
	case reflect.Pointer:
		child, err := getFieldDecoder(t.Elem())
		return pointerDecoder{child}, err
	case reflect.String, reflect.Uint64, reflect.Int64, reflect.Uint32, reflect.Int32:
		return rawFieldDecoder{}, nil
	case reflect.Struct:
		fallthrough
	default:
		return nil, fmt.Errorf("unsupported attribute type %q", t)
	}
}

type recordAttrFieldDecoder struct {
}

func (d recordAttrFieldDecoder) decode(attr interface{}, v reflect.Value) error {
	if v.Kind() == reflect.Pointer && v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	if v.Kind() != reflect.Pointer &&
		v.CanAddr() &&
		reflect.PointerTo(v.Type()).Implements(recordAttributeDecoderType) {
		v = v.Addr()
	}
	obj := v.Interface()
	decoder, ok := obj.(RecordAttributeDecoder)
	if !ok {
		panic(fmt.Sprintf("expected %T to implement RecordAttributeUnmarshaler", obj))
	}
	return decoder.DecodeRecordAttribute(attr)
}

type rawFieldDecoder struct{}

func (d rawFieldDecoder) decode(attr interface{}, v reflect.Value) error {
	if !v.CanSet() {
		return fmt.Errorf("field not addressable")
	}
	attrV := reflect.ValueOf(attr)
	if !v.Type().AssignableTo(attrV.Type()) {
		return fmt.Errorf("cannot assign value to type")
	}
	v.Set(attrV)
	return nil
}

type pointerDecoder struct {
	sub fieldDecoder
}

func (d pointerDecoder) decode(attr interface{}, v reflect.Value) error {
	if !v.CanSet() {
		return fmt.Errorf("field not addressable")
	}
	if v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	if err := d.sub.decode(attr, v.Elem()); err != nil {
		return err
	}
	return nil
}
