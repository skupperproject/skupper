package encoding_test

import (
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/encoding"
	"gotest.tools/assert"
)

type tRecordAttributeEncodeDecode struct {
	A *MagicBool `vflow:"99"`
	B MagicBool  `vflow:"100"`
}

type MagicBool bool

func (m MagicBool) EncodeRecordAttribute() (interface{}, error) {
	if m {
		return "OKAY", nil
	}
	return "ERROR", nil
}

func (m *MagicBool) DecodeRecordAttribute(obj interface{}) error {
	if obj.(string) == "OKAY" {
		(*m) = true
	}
	return nil
}

const (
	recordTypeUnused uint32 = iota + 99900
	recordTypeRecordAttributeEncodeDecode
)

func init() {
	encoding.MustRegisterRecord(recordTypeRecordAttributeEncodeDecode, tRecordAttributeEncodeDecode{})
}

func TestMustRegister(t *testing.T) {
	expectPanic := func(t *testing.T, expected string) func() {
		return func() {
			if recovered := recover(); recovered != nil {
				assert.Check(t, strings.HasPrefix(recovered.(string), expected), "got %q but expected %q", recovered, expected)
				return
			}
			t.Fatal("expected MustRegister to panic")
		}
	}
	t.Run("existing codepoint", func(t *testing.T) {
		defer expectPanic(t, "cannot register record type struct {} using codepoint 99901: already in use")()
		encoding.MustRegisterRecord(recordTypeRecordAttributeEncodeDecode, struct{}{})
	})
	t.Run("repeated vflow attribute tags", func(t *testing.T) {
		defer expectPanic(t, `struct field B repeats vflow tag "1" also used by A`)()
		type Repeat struct {
			A int64 `vflow:"1"`
			B int64 `vflow:"1"`
		}
		encoding.MustRegisterRecord(recordTypeUnused, Repeat{})
	})
	t.Run("invalid tag", func(t *testing.T) {
		defer expectPanic(t, `vflow struct tag parse error for field A:`)()
		type Repeat struct {
			A int64 `vflow:"identity"`
		}
		encoding.MustRegisterRecord(recordTypeUnused, Repeat{})
	})
	t.Run("invalid type", func(t *testing.T) {
		defer expectPanic(t, `invalid vflow field encoder for "D": unsupported attribute type "testing.B"`)()
		type Invalid struct {
			A testing.B
			B *testing.B
			c testing.B `vflow:"3"`
			D testing.B `vflow:"4"`
		}
		encoding.MustRegisterRecord(recordTypeUnused, Invalid{c: testing.B{}})
	})
	t.Run("register twice", func(t *testing.T) {
		defer expectPanic(t, `cannot register same type more than once. type encoding_test.tRecordAttributeEncodeDecode already registered with code`)()
		encoding.MustRegisterRecord(recordTypeUnused, tRecordAttributeEncodeDecode{})
	})
}

func TestMarshal(t *testing.T) {
	timeA := vanflow.Time{Time: time.UnixMicro(100)}
	timeB := vanflow.Time{Time: time.UnixMicro(333)}
	testCases := []struct {
		Name          string
		Input         interface{}
		ExpectErr     bool
		ExpectedAttrs map[interface{}]interface{}
	}{
		{
			Name:      "missing identity error",
			Input:     vanflow.SiteRecord{},
			ExpectErr: true,
		}, {
			Name:      "nil",
			ExpectErr: true,
		}, {
			Name:  "basic site",
			Input: vanflow.SiteRecord{BaseRecord: vanflow.BaseRecord{ID: "test"}},
			ExpectedAttrs: map[any]any{
				uint32(0): uint32(0),
				uint32(1): "test",
			},
		}, {
			Name: "full site",
			Input: vanflow.SiteRecord{
				Location: ptrTo("loc"),
				BaseRecord: vanflow.BaseRecord{
					ID:        "test",
					StartTime: ptrTo(timeA),
					EndTime:   ptrTo(timeB),
				}},
			ExpectedAttrs: map[any]any{
				uint32(0): uint32(0),
				uint32(1): "test",
				uint32(3): uint64(100),
				uint32(4): uint64(333),
				uint32(9): "loc",
			},
		}, {
			Name: "RecordAttributeMarshaler",
			Input: ptrTo(tRecordAttributeEncodeDecode{
				A: ptrTo(MagicBool(true)),
				B: MagicBool(false),
			}),
			ExpectedAttrs: map[any]any{
				uint32(0):   recordTypeRecordAttributeEncodeDecode, // type
				uint32(99):  "OKAY",
				uint32(100): "ERROR",
			},
		}, {
			Name:      "unregistered type",
			Input:     testing.B{},
			ExpectErr: true,
		}, {
			Name:      "time before epoch",
			Input:     vanflow.SiteRecord{BaseRecord: vanflow.NewBase("test", time.UnixMicro(-1))},
			ExpectErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			actual, err := encoding.Encode(tc.Input)
			if tc.ExpectErr {
				assert.Check(t, err != nil)
			} else {
				assert.Check(t, err)
				assert.DeepEqual(t, tc.ExpectedAttrs, map[any]any(actual))
			}
		})
	}
}

func BenchmarkEncodeDecode(b *testing.B) {
	timeA := vanflow.Time{Time: time.UnixMicro(100)}
	timeB := vanflow.Time{Time: time.UnixMicro(333)}
	exemplar := vanflow.SiteRecord{
		Location: ptrTo("loc"),
		Platform: ptrTo("kubernetes"),
		Name:     ptrTo("test"),
		BaseRecord: vanflow.BaseRecord{
			ID:        "test",
			StartTime: ptrTo(timeA),
			EndTime:   ptrTo(timeB),
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attrs, _ := encoding.Encode(exemplar)
		_, err := encoding.Decode(attrs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func ptrTo[T any](t T) *T {
	return &t
}
