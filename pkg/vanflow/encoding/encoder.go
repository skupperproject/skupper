package encoding

import (
	"errors"
	"fmt"
	"reflect"
)

type encodingError string

func (err encodingError) Error() string {
	return string(err)
}

const (
	// ErrAttributeNotSet can be returned from a RecordAttributeEncoder
	// when the field should not be set
	ErrAttributeNotSet = encodingError("record attribute not set")
)

var (
	recordAttributeEncoderType = reflect.TypeOf((*RecordAttributeEncoder)(nil)).Elem()
)

type RecordAttributeSet = map[interface{}]interface{}

// RecordAttributeEncoder is implemented by record attribute types to overwrite
// the default encoding behavior
type RecordAttributeEncoder interface {
	EncodeRecordAttribute() (interface{}, error)
}

// Encode a record into a record attribute set so that it can be sent over the
// vanflow protocol. Only records types that have been registered with
// MustRegisterRecord can be encoded.
func Encode(record any) (RecordAttributeSet, error) {
	mu.RLock()
	defer mu.RUnlock()
	if record == nil {
		return nil, errors.New("cannot encode nil record")
	}
	recordV := reflect.ValueOf(record)
	recordT := recordV.Type()
	encoding, ok := encodings[recordT]
	if !ok {
		if recordT.Kind() == reflect.Pointer {
			recordV = recordV.Elem()
			recordT = recordV.Type()
			encoding, ok = encodings[recordT]
		}
		if !ok {
			return nil, fmt.Errorf("encode error: unregistered record type %T", record)
		}
	}
	result, err := encoding.encode(recordV)
	if result != nil {
		result[typeOfRecord] = encoding.codepoint
	}
	return result, err
}

type fieldEncoder interface {
	encode(v reflect.Value) (interface{}, error)
}

func getFieldEncoder(t reflect.Type) (fieldEncoder, error) {
	if t.Implements(recordAttributeEncoderType) {
		return recordAttrFieldEncoder{}, nil
	}
	switch t.Kind() {
	case reflect.Pointer:
		child, err := getFieldEncoder(t.Elem())
		return pointerEncoder{child}, err
	case reflect.String, reflect.Uint64, reflect.Int64, reflect.Uint32, reflect.Int32:
		return rawFieldEncoder{}, nil
	case reflect.Struct:
		fallthrough
	default:
		return nil, fmt.Errorf("unsupported attribute type %q", t)
	}
}

type rawFieldEncoder struct {
}

func (e rawFieldEncoder) encode(v reflect.Value) (interface{}, error) {
	if v.IsZero() {
		return nil, ErrAttributeNotSet
	}
	return v.Interface(), nil
}

type pointerEncoder struct {
	sub fieldEncoder
}

func (e pointerEncoder) encode(v reflect.Value) (interface{}, error) {
	if v.IsNil() {
		return nil, ErrAttributeNotSet
	}
	return e.sub.encode(v.Elem())
}

type recordAttrFieldEncoder struct {
}

func (e recordAttrFieldEncoder) encode(v reflect.Value) (interface{}, error) {
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return nil, ErrAttributeNotSet
	}
	val := v.Interface()
	encoder, ok := val.(RecordAttributeEncoder)
	if !ok {
		panic(fmt.Sprintf("encoder error: type %s does not implement RecordAttributeEncoder", v.Type()))
	}
	return encoder.EncodeRecordAttribute()
}
