package flow

import (
	"net/http"
	"testing"

	"gotest.tools/assert"
)

func TestTimeRange(t *testing.T) {
	type test struct {
		name               string
		startTime          uint64
		endTime            uint64
		queryParams        QueryParams
		timeRangeStart     uint64
		timeRangeEnd       uint64
		timeRangeOperation TimeRangeRelation
		result             bool
	}

	testTable := []test{
		// A[2,3]
		{
			name:      "A-intersects",
			startTime: 2,
			endTime:   3,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: intersects,
			},
			result: false,
		},
		{
			name:      "A-contains",
			startTime: 2,
			endTime:   3,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: contains,
			},
			result: false,
		},
		{
			name:      "A-within",
			startTime: 2,
			endTime:   3,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: within,
			},
			result: false,
		},
		// B[5,7]
		{
			name:      "B-intersects",
			startTime: 5,
			endTime:   7,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: intersects,
			},
			result: true,
		},
		{
			name:      "B-contains",
			startTime: 5,
			endTime:   7,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: contains,
			},
			result: false,
		},
		{
			name:      "B-within",
			startTime: 5,
			endTime:   7,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: within,
			},
			result: false,
		},
		// C[8,11]
		{
			name:      "C-intersects",
			startTime: 8,
			endTime:   11,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: intersects,
			},
			result: true,
		},
		{
			name:      "C-contains",
			startTime: 8,
			endTime:   11,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: contains,
			},
			result: false,
		},
		{
			name:      "C-within",
			startTime: 8,
			endTime:   11,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: within,
			},
			result: true,
		},
		// D[12,14]
		{
			name:      "D-intersects",
			startTime: 12,
			endTime:   14,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: intersects,
			},
			result: true,
		},
		{
			name:      "D-contains",
			startTime: 12,
			endTime:   14,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: contains,
			},
			result: false,
		},
		{
			name:      "D-within",
			startTime: 12,
			endTime:   14,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: within,
			},
			result: false,
		},
		// E[12,14]
		{
			name:      "E-intersects",
			startTime: 16,
			endTime:   18,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: intersects,
			},
			result: false,
		},
		{
			name:      "E-contains",
			startTime: 16,
			endTime:   18,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: contains,
			},
			result: false,
		},
		{
			name:      "E-within",
			startTime: 16,
			endTime:   18,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: within,
			},
			result: false,
		},
		// F[4,16]
		{
			name:      "F-intersects",
			startTime: 4,
			endTime:   16,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: intersects,
			},
			result: true,
		},
		{
			name:      "F-contains",
			startTime: 4,
			endTime:   16,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: contains,
			},
			result: true,
		},
		{
			name:      "F-within",
			startTime: 4,
			endTime:   16,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: within,
			},
			result: false,
		},
		// G[9,0]
		{
			name:      "G-intersects",
			startTime: 9,
			endTime:   0,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: intersects,
			},
			result: true,
		},
		{
			name:      "G-contains",
			startTime: 9,
			endTime:   0,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: contains,
			},
			result: false,
		},
		{
			name:      "G-within",
			startTime: 9,
			endTime:   0,
			queryParams: QueryParams{
				TimeRangeStart:     6,
				TimeRangeEnd:       13,
				TimeRangeOperation: within,
			},
			result: false,
		},
	}

	for _, test := range testTable {
		base := Base{
			StartTime: test.startTime,
			EndTime:   test.endTime,
		}
		result := base.TimeRangeValid(test.queryParams)
		assert.Equal(t, test.result, result, test.name)
	}
}

func TestParameters(t *testing.T) {
	type test struct {
		values      map[string]string
		queryParams QueryParams
	}

	testTable := []test{
		{
			values: map[string]string{"timeRangeStart": "1234", "timeRangeEnd": "0"},
			queryParams: QueryParams{
				Offset:             -1,
				Limit:              -1,
				SortBy:             "identity.asc",
				Filter:             "",
				TimeRangeStart:     uint64(1234),
				TimeRangeEnd:       uint64(0),
				TimeRangeOperation: intersects,
			},
		},
		{
			values: map[string]string{"timeRangeStart": "0", "timeRangeEnd": "0", "timeRangeOperation": "contains"},
			queryParams: QueryParams{
				Offset:             -1,
				Limit:              -1,
				SortBy:             "identity.asc",
				Filter:             "",
				TimeRangeStart:     uint64(0),
				TimeRangeEnd:       uint64(0),
				TimeRangeOperation: contains,
			},
		},
		{
			values: map[string]string{"timeRangeStart": "1234", "timeRangeEnd": "0", "timeRangeOperation": "within"},
			queryParams: QueryParams{
				Offset:             -1,
				Limit:              -1,
				SortBy:             "identity.asc",
				Filter:             "",
				TimeRangeStart:     uint64(1234),
				TimeRangeEnd:       uint64(0),
				TimeRangeOperation: within,
			},
		},
		{
			values: map[string]string{
				"offset":             "10",
				"limit":              "10",
				"sortBy":             "sourcePort.desc",
				"filter":             "forwardFlow.protocol.tcp",
				"timeRangeStart":     "0",
				"timeRangeEnd":       "4567",
				"timeRangeOperation": "intersects",
			},
			queryParams: QueryParams{
				Offset:             10,
				Limit:              10,
				SortBy:             "sourcePort.desc",
				Filter:             "forwardFlow.protocol.tcp",
				TimeRangeStart:     uint64(0),
				TimeRangeEnd:       uint64(4567),
				TimeRangeOperation: intersects,
			},
		},
	}

	for _, test := range testTable {
		req, _ := http.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		for k, v := range test.values {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
		qp := getQueryParams(req.URL)
		assert.Equal(t, qp.Offset, test.queryParams.Offset)
		assert.Equal(t, qp.Limit, test.queryParams.Limit)
		assert.Equal(t, qp.SortBy, test.queryParams.SortBy)
		assert.Equal(t, qp.TimeRangeStart, test.queryParams.TimeRangeStart)
		assert.Equal(t, qp.TimeRangeEnd, test.queryParams.TimeRangeEnd)
		assert.Equal(t, qp.TimeRangeOperation, test.queryParams.TimeRangeOperation)
	}
}

func TestPagination(t *testing.T) {
	type test struct {
		offset int
		limit  int
		length int
		start  int
		end    int
	}

	testTable := []test{
		{
			offset: -1,
			limit:  -1,
			length: 100,
			start:  0,
			end:    100,
		},
		{
			offset: 0,
			limit:  10,
			length: 100,
			start:  0,
			end:    10,
		},
		{
			offset: 90,
			limit:  20,
			length: 100,
			start:  90,
			end:    100,
		},
		{
			offset: 110,
			limit:  20,
			length: 100,
			start:  100,
			end:    100,
		},
	}

	for _, test := range testTable {
		start, end := paginate(test.offset, test.limit, test.length)
		assert.Equal(t, test.start, start)
		assert.Equal(t, test.end, end)
	}
}

func TestMatchField(t *testing.T) {
	field1 := "foo"
	field1Value := "foo"
	field2 := uint64(12345678)
	field2Value := "12345678"
	field3 := int32(87654321)
	field3Value := "87654321"
	field4 := int64(12345678)
	field4Value := "12345678"
	field5 := int(12345678)
	field5Value := "12345678"

	match := matchFieldValue(field1, field1Value)
	assert.Equal(t, match, true)
	match = matchFieldValue(field2, field2Value)
	assert.Equal(t, match, true)
	match = matchFieldValue(field3, field3Value)
	assert.Equal(t, match, true)
	match = matchFieldValue(field4, field4Value)
	assert.Equal(t, match, true)
	match = matchFieldValue(field5, field5Value)
	assert.Equal(t, match, true)
	match = matchFieldValue(field5, field1Value)
	assert.Equal(t, match, false)
}

func TestCompareFields(t *testing.T) {
	type test struct {
		field1 interface{}
		field2 interface{}
		order  string
		result bool
	}

	testTable := []test{
		{
			field1: "foo",
			field2: "bar",
			order:  "asc",
			result: false,
		},
		{
			field1: "foo",
			field2: "bar",
			order:  "desc",
			result: true,
		},
		{
			field1: uint64(12345678),
			field2: uint64(87654321),
			order:  "asc",
			result: true,
		},
		{
			field1: uint64(12345678),
			field2: uint64(87654321),
			order:  "desc",
			result: false,
		},
		{
			field1: int32(12345678),
			field2: int32(87654321),
			order:  "asc",
			result: true,
		},
		{
			field1: int32(12345678),
			field2: int32(87654321),
			order:  "desc",
			result: false,
		},
		{
			field1: int64(12345678),
			field2: int64(87654321),
			order:  "asc",
			result: true,
		},
		{
			field1: int64(12345678),
			field2: int64(87654321),
			order:  "desc",
			result: false,
		},
		{
			field1: int(12345678),
			field2: int(87654321),
			order:  "asc",
			result: true,
		},
		{
			field1: int(12345678),
			field2: int(87654321),
			order:  "desc",
			result: false,
		},
	}

	for _, test := range testTable {
		result := compareFields(test.field1, test.field2, test.order)
		assert.Equal(t, test.result, result)
	}
}

func TestFilterRecord(t *testing.T) {
	host := "10.20.30.40"
	name := "public1"
	octets := uint64(2030)
	flow := FlowRecord{
		SourceHost: &host,
		Octets:     &octets,
	}
	flowPair := FlowPairRecord{
		Base: Base{
			Identity: "foo",
		},
		SourceSiteName: &name,
		ForwardFlow:    &flow,
	}

	type test struct {
		filter string
		result bool
	}
	testTable := []test{
		{
			filter: "forwardFlow.SourceHost.10.20.30.40",
			result: true,
		},
		{
			filter: "sourceSiteName.public1",
			result: true,
		},
		{
			filter: "forwardFlow.Octets.2030",
			result: true,
		},
		{
			filter: "identity.foo",
			result: true,
		},
		{
			filter: "identity.bar",
			result: false,
		},
		{
			filter: "",
			result: true,
		},
		{
			filter: "identity",
			result: false,
		},
	}

	for _, test := range testTable {
		result := filterRecord(flowPair, test.filter)
		assert.Equal(t, result, test.result)
	}
}

func TestSortAndSlice(t *testing.T) {
	identity1 := "foo"
	host1 := "10.20.30.40"
	name1 := "public1"
	octets1 := uint64(2030)
	flow1 := FlowRecord{
		SourceHost: &host1,
		Octets:     &octets1,
	}
	flowPair1 := FlowPairRecord{
		Base: Base{
			Identity: identity1,
		},
		SourceSiteName: &name1,
		ForwardFlow:    &flow1,
	}

	identity2 := "bar"
	host2 := "10.20.30.41"
	name2 := "public2"
	octets2 := uint64(2031)
	flow2 := FlowRecord{
		SourceHost: &host2,
		Octets:     &octets2,
	}
	flowPair2 := FlowPairRecord{
		Base: Base{
			Identity: identity2,
		},
		SourceSiteName: &name2,
		ForwardFlow:    &flow2,
	}

	type test struct {
		payload  *Payload
		identity string
		result   error
	}
	testTable := []test{
		{
			payload: &Payload{
				QueryParams: QueryParams{
					Offset: 0,
					Limit:  10,
					SortBy: "identity.asc",
				},
			},
			identity: identity2,
			result:   nil,
		},
		{
			payload: &Payload{
				QueryParams: QueryParams{
					Offset: 0,
					Limit:  10,
					SortBy: "identity.desc",
				},
			},
			identity: identity1,
			result:   nil,
		},
		{
			payload: &Payload{
				QueryParams: QueryParams{
					Offset: 0,
					Limit:  10,
					SortBy: "forwardFlow.sourceHost.asc",
				},
			},
			identity: identity1,
			result:   nil,
		},
		{
			payload: &Payload{
				QueryParams: QueryParams{
					Offset: 0,
					Limit:  10,
					SortBy: "forwardFlow.sourceHost.desc",
				},
			},
			identity: identity2,
			result:   nil,
		},
	}

	for _, test := range testTable {
		fps := []FlowPairRecord{flowPair1, flowPair2}
		ok := sortAndSlice(fps, test.payload)
		assert.Equal(t, test.result, ok)
		assert.Equal(t, fps[0].Identity, test.identity)
	}
}
