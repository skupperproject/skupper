package encoding_test

import (
	"fmt"
	"math"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/encoding"
)

type tNestedEmbedded struct {
	A
	Root uint32 `vflow:"1"`
}
type A struct {
	B
	X
}
type B struct {
	B string `vflow:"100,required"`
}
type X struct {
	X string `vflow:"10,required"`
}

type tAllAttributeTypes struct {
	S    string  `vflow:"1"`
	Sp   *string `vflow:"2"`
	U32  uint32  `vflow:"3"`
	U32p *uint32 `vflow:"4"`
	U64  uint64  `vflow:"5"`
	I32  int32   `vflow:"6"`
	I64  int64   `vflow:"7"`
	I64p *int64  `vflow:"8"`
}

const (
	recordTypeNestedEmbedded uint32 = iota + 88801
	recordTypeAllAttributeTypes
)

func init() {
	encoding.MustRegisterRecord(recordTypeNestedEmbedded, tNestedEmbedded{})
	encoding.MustRegisterRecord(recordTypeAllAttributeTypes, tAllAttributeTypes{})
}

var decoderTests = []struct {
	CaseName
	In       map[any]any
	ErrorMsg string
	Out      interface{}
	Golden   bool
}{
	{
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): uint32(1), uint32(1): "routerid", uint32(2): "parentid", uint32(3): uint64(100), uint32(4): uint64(1000),
			uint32(12): "default", uint32(30): "routername",
		},
		Out: vanflow.RouterRecord{
			BaseRecord: vanflow.NewBase("routerid", time.UnixMicro(100), time.UnixMicro(1000)),
			Parent:     ptrTo("parentid"),
			Namespace:  ptrTo("default"),
			Name:       ptrTo("routername"),
		},
		Golden: true,
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): uint32(1), uint32(1): "routerid", uint32(2): "parentid", uint32(3): uint64(100), uint32(4): uint64(1000),
			uint32(12): "default", uint32(30): "routername",
		},
		Out: vanflow.RouterRecord{
			BaseRecord: vanflow.NewBase("routerid", time.UnixMicro(100), time.UnixMicro(1000)),
			Parent:     ptrTo("parentid"),
			Namespace:  ptrTo("default"),
			Name:       ptrTo("routername"),
		},
	}, {
		CaseName: Name(""),
		ErrorMsg: "decode error: cannot decode nil record attribute set",
	}, {
		CaseName: Name(""),
		In:       func() map[any]any { var nilMap map[any]any; return nilMap }(),
		ErrorMsg: "decode error: cannot decode nil record attribute set",
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): uint32(404_404), uint32(12): "unknown",
		},
		ErrorMsg: "decode error: unknown record type for 404404",
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): uint32(404_404), 12: "unknown",
		},
		ErrorMsg: "decode error: unknown record type for 404404",
	}, {
		CaseName: Name(""),
		In: map[any]any{
			int(0): uint32(404_404), 12: "unknown",
		},
		ErrorMsg: "decode error: record type attribute not present",
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(12): "unknown",
		},
		ErrorMsg: "decode error: record type attribute not present",
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): "unknown",
		},
		ErrorMsg: `decode error: unexpected type for record type attribute "string"`,
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): uint32(1),
		},
		ErrorMsg: `decode error: record attribute set missing required field "ID"`,
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): uint32(1), int64(1): "incorrect key type",
		},
		ErrorMsg: `decode error: record attribute set missing required field "ID"`,
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): recordTypeRecordAttributeEncodeDecode, uint32(99): "OKAY", uint32(100): "OKAY",
		},
		Out:    tRecordAttributeEncodeDecode{A: ptrTo(MagicBool(true)), B: MagicBool(true)},
		Golden: true,
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): recordTypeRecordAttributeEncodeDecode,
		},
		Out: tRecordAttributeEncodeDecode{},
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): uint32(1), uint32(1): "routerid",
			uint32(4): uint64(math.MaxInt64) + 1,
		},
		ErrorMsg: `decode error: error decoding field "EndTime": time too far in future for internal representation: 0x8000000000000000`,
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): recordTypeNestedEmbedded, uint32(1): uint32(1), uint32(10): "ten", uint32(100): "100",
		},
		Out: tNestedEmbedded{Root: 1, A: A{B: B{B: "100"}, X: X{X: "ten"}}},
	}, {
		CaseName: Name(""),
		In: map[any]any{
			uint32(0): uint32(1), uint32(1): "routerid", uint32(3): uint64(100),
		},
		Out: vanflow.RouterRecord{
			BaseRecord: vanflow.NewBase("routerid", time.UnixMicro(100)),
		},
		Golden: true,
	}, {
		CaseName: Name("all primitive attributes empty"),
		In: map[any]any{
			uint32(0): recordTypeAllAttributeTypes,
		},
		Out:    tAllAttributeTypes{},
		Golden: true,
	}, {
		CaseName: Name("all primitive attributes full"),
		In: map[any]any{
			uint32(0): recordTypeAllAttributeTypes,
			uint32(1): "str", uint32(2): "strP",
			uint32(3): uint32(10), uint32(4): uint32(10),
			uint32(5): uint64(20),
			uint32(6): int32(20),
			uint32(7): int64(30), uint32(8): int64(30),
		},
		Out: tAllAttributeTypes{
			S: "str", Sp: ptrTo("strP"), U32: 10, U32p: ptrTo(uint32(10)), I32: 20, U64: 20, I64: 30, I64p: ptrTo(int64(30)),
		},
		Golden: true,
	},
}

func TestDecode(t *testing.T) {
	for _, tc := range decoderTests {
		t.Run(tc.Name, func(t *testing.T) {
			out, err := encoding.Decode(tc.In)
			if !equalError(err, tc.ErrorMsg) {
				t.Fatalf("%s: unexpected error. wanted: %q but got: %q", tc.Where, tc.ErrorMsg, err)
			}
			if tc.Out == nil {
				return
			}

			if !cmp.Equal(tc.Out, out) {
				t.Fatalf("%s: Decode got: %+v want: %+v\n\n%s", tc.Where, out, tc.Out, cmp.Diff(tc.Out, out))
			}

			if tc.ErrorMsg != "" || !tc.Golden {
				return
			}

			remarshaled, err := encoding.Encode(out)
			if err != nil {
				t.Fatalf("%s: unexpected encode error: %q", tc.Where, err)
			}
			if !cmp.Equal(tc.In, remarshaled) {
				t.Fatalf("%s: Encode got: %+v want: %+v\n\n%s", tc.Where, remarshaled, tc.In, cmp.Diff(tc.In, remarshaled))
			}

		})
	}
}

func equalError(e error, s string) bool {
	if e == nil || s == "" {
		return e == nil && s == ""
	}
	return e.Error() == s
}

type CaseName struct {
	Name  string
	Where CasePos
}

func Name(name string) CaseName {
	c := CaseName{Name: name}
	runtime.Callers(2, c.Where[:])
	return c
}

type CasePos [1]uintptr

func (p CasePos) String() string {
	frames := runtime.CallersFrames(p[:])
	next, _ := frames.Next()
	return fmt.Sprintf("%s:%d", path.Base(next.File), next.Line)
}
