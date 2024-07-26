package vanflow

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow/encoding"
	"gotest.tools/assert"
)

func TestTimeEncoding(t *testing.T) {

	now := time.Now().Truncate(time.Microsecond)
	testCases := []struct {
		Name           string
		In             any
		DecodeError    string
		EncodeError    string
		NotDeeplyEqual bool
		HandleOutput   func(*testing.T, map[interface{}]interface{})
	}{
		{
			Name: "basic",
			In: LinkRecord{
				BaseRecord: NewBase("i", now),
			},
			HandleOutput: func(t *testing.T, attrs map[interface{}]interface{}) {
				assert.Equal(t, attrs[uint32(3)], uint64(now.UnixMicro()))
			},
		}, {
			Name: "before epoch",
			In: LinkRecord{
				BaseRecord: NewBase("i", time.UnixMicro(-1)),
			},
			DecodeError: `cannot represent times before epoch in this encoding`,
		}, {
			Name: "too large",
			In: LinkRecord{
				BaseRecord: BaseRecord{ID: "i"},
			},
			HandleOutput: func(t *testing.T, attrs map[interface{}]interface{}) {
				_, ok := attrs[uint32(3)]
				assert.Assert(t, !ok)
				attrs[uint32(3)] = uint64(1 << 63) // too large for int64
			},
			EncodeError: `time too far in future for internal representation`,
		}, {
			Name: "unexpected type",
			In: LinkRecord{
				BaseRecord: BaseRecord{ID: "i"},
			},
			HandleOutput: func(t *testing.T, attrs map[interface{}]interface{}) {
				_, ok := attrs[uint32(3)]
				assert.Assert(t, !ok)
				attrs[uint32(3)] = "yahoo"
			},
			EncodeError: `expected type uint64 for timestamp but got string`,
		}, {
			Name:           "Ignores Zero Time",
			NotDeeplyEqual: true,
			In: LinkRecord{
				BaseRecord: NewBase("i", time.Time{}, time.UnixMicro(0)),
			},
			HandleOutput: func(t *testing.T, attrs map[interface{}]interface{}) {
				_, ok := attrs[uint32(3)]
				assert.Assert(t, !ok)
				endTime, ok := attrs[uint32(4)]
				assert.Assert(t, ok)
				assert.Equal(t, endTime, uint64(0))
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			out, err := encoding.Encode(tc.In)
			if tc.DecodeError != "" {
				assert.ErrorContains(t, err, tc.DecodeError)
				return
			}
			assert.Check(t, err)
			if tc.HandleOutput != nil {
				tc.HandleOutput(t, out)
			}
			duplicate, err := encoding.Decode(out)
			if tc.EncodeError != "" {
				assert.ErrorContains(t, err, tc.EncodeError)
				return
			}
			assert.Check(t, err)
			if tc.NotDeeplyEqual {
				return
			}
			assert.DeepEqual(t, tc.In, duplicate)
		})
	}

}
