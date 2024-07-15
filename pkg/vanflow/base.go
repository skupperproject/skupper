package vanflow

import (
	"fmt"
	"math"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow/encoding"
)

type TypeMeta struct {
	APIVersion string
	Type       string
}

func (m TypeMeta) String() string {
	return fmt.Sprintf("%s/%s", m.APIVersion, m.Type)
}

type Record interface {
	Identity() string
	GetTypeMeta() TypeMeta
}

type BaseRecord struct {
	ID        string `vflow:"1,required"`
	StartTime *Time  `vflow:"3"`
	EndTime   *Time  `vflow:"4"`
}

func NewBase(id string, times ...time.Time) BaseRecord {
	base := BaseRecord{ID: id}
	nextTime := func() *Time {
		if len(times) > 0 {
			var first Time
			first, times = Time{times[0]}, times[1:]
			return &first
		}
		return nil
	}
	base.StartTime = nextTime()
	base.EndTime = nextTime()
	if add := nextTime(); add != nil {
		panic("expected at most two times for start and end")
	}
	return base
}
func (b BaseRecord) Identity() string {
	return b.ID
}

type Time struct {
	time.Time
}

func (t Time) EncodeRecordAttribute() (any, error) {
	if t.Time.IsZero() {
		return nil, encoding.ErrAttributeNotSet
	}
	ts := t.Time.UnixMicro()
	if ts < 0 {
		return nil, fmt.Errorf("cannot represent times before epoch in this encoding: %d", ts)
	}
	return uint64(ts), nil
}

func (t *Time) DecodeRecordAttribute(attr any) error {
	uintAttr, ok := attr.(uint64)
	if !ok {
		return fmt.Errorf("expected type uint64 for timestamp but got %T", attr)
	}
	if uintAttr > math.MaxInt64 { // undefined past the year 294246
		return fmt.Errorf("time too far in future for internal representation: %#x", uintAttr)
	}
	t.Time = time.UnixMicro(int64(uintAttr))
	return nil
}
