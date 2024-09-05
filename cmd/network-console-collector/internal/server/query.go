package server

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func filterAndOrderResults[T api.Record](r *http.Request, results []T) ([]T, int64, error) {
	var (
		out            []T = results
		isCopy         bool
		timeRangeCount int64
	)

	qp := getQueryParams(r)

	filterFields := make(map[string]fieldIndex[T], len(qp.FilterFields))
	for path := range qp.FilterFields {
		m, err := indexerForField[T](path)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid filter parameter %q for record type %T", path, results)
		}
		filterFields[path] = m
	}

	for i, item := range results {
		matches := true
		for path, values := range qp.FilterFields {
			if !filterFields[path].MatchesFilter(item, values) {
				matches = false
				break
			}
		}
		switch {
		case matches && !isCopy:
			continue
		case isCopy && matches:
			out = append(out, item)
		case !matches && !isCopy:
			isCopy = true
			out = make([]T, 0, len(results)-1)
			out = append(out, results[:i]...)
		}
	}

	var st T
	if _, ok := any(st).(api.Record); ok {
		out = filterTime(out, qp.State, qp.TimeRangeOperation, qp.TimeRangeStart, qp.TimeRangeEnd)
	}

	// record count after filtering but before pagination/limiting
	timeRangeCount = int64(len(out))

	sortFieldIndexer, err := indexerForField[T](qp.SortField)
	if err != nil {
		return out[:0], timeRangeCount, fmt.Errorf("invalid sortBy parameter %q: %s", r.URL.Query().Get("sortBy"), err)
	}

	offset := qp.Offset
	limit := qp.Limit
	start := 0
	end := 0
	sort.Slice(out, func(i, j int) bool {
		d := sortFieldIndexer.Compare(out[i], out[j])
		if qp.SortDescending {
			return d > 0
		}
		return d < 0
	})
	start, end = paginate(offset, limit, len(out))
	out = out[start:end]

	return out, timeRangeCount, nil
}

func filterTime[T api.Record](all []T, state timeRangeState, op timeRangeRelation, rangeStart, rangeEnd uint64) []T {
	var (
		out    = all
		isCopy bool
	)

	shouldFilterOp := func(t api.Record) bool { return false }
	switch op {
	case intersects:
		shouldFilterOp = func(t api.Record) bool {
			start, end := t.GetStartTime(), t.GetEndTime()
			return (end != 0 && end < rangeStart || start > rangeEnd)
		}
	case contains:
		shouldFilterOp = func(t api.Record) bool {
			start, end := t.GetStartTime(), t.GetEndTime()
			return !(start <= rangeStart && (end == 0 || end >= rangeEnd))
		}
	case within:
		shouldFilterOp = func(t api.Record) bool {
			start, end := t.GetStartTime(), t.GetEndTime()
			return !(start >= rangeStart && (end != 0 && end <= rangeEnd))
		}
	}
	shouldFilter := func(t api.Record) bool { return shouldFilterOp(t) }
	switch state {
	case active:
		shouldFilter = func(record api.Record) bool {
			if record.GetEndTime() != 0 {
				return true
			}
			return shouldFilterOp(record)
		}
	case terminated:
		shouldFilter = func(record api.Record) bool {
			if record.GetEndTime() == 0 {
				return true
			}
			return shouldFilterOp(record)
		}
	}

	for i, record := range all {
		toRemove := shouldFilter(record)
		switch {
		case !toRemove && !isCopy:
			continue
		case isCopy && !toRemove:
			out = append(out, record)
		case toRemove && !isCopy:
			isCopy = true
			out = make([]T, 0, len(all)-1)
			out = append(out, all[:i]...)
		}
	}

	return out
}

type fieldIndex[T any] struct {
	index []int
}

func (m fieldIndex[T]) Compare(x, y T) int {
	vx := reflect.ValueOf(x).FieldByIndex(m.index)
	vy := reflect.ValueOf(y).FieldByIndex(m.index)
	if vx.Kind() == reflect.Pointer {
		switch {
		case vx.IsNil() && vy.IsNil():
			return 0
		case !vx.IsNil() && vy.IsNil():
			return 1
		case vx.IsNil() && !vy.IsNil():
			return -1
		}
		vx, vy = vx.Elem(), vy.Elem()
	}
	xx, yy := vx.Interface(), vy.Interface()
	switch xx := xx.(type) {
	case string:
		yy := yy.(string)
		if xx == yy {
			return 0
		}
		if xx < yy {
			return -1
		}
		return 1
	case uint64:
		return int(xx - yy.(uint64))
	case int32:
		return int(xx - yy.(int32))
	case int64:
		return int(xx - yy.(int64))
	case int:
		return int(xx - yy.(int))
	}
	return 0
}

func (m fieldIndex[T]) MatchesFilter(e T, values []string) bool {
	val := reflect.ValueOf(e).FieldByIndex(m.index)
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return false
		}
		val = val.Elem()
	}
	switch val.Kind() {
	case reflect.String:
		target := val.String()
		for _, y := range values {
			if y == "" {
				continue
			}
			if strings.HasPrefix(target, y) {
				return true
			}
		}
	case reflect.Uint64:
		return numInStringSlice(val.Uint(), values)
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Int:
		return numInStringSlice(val.Int(), values)
	case reflect.Bool:
		return boolInStringSlice(val.Bool(), values)
	}
	return false
}

func indexerForField[T any](fieldPath string) (fieldIndex[T], error) {
	var indexer fieldIndex[T]
	parts := strings.Split(fieldPath, ".")
	for i := range parts {
		parts[i] = cases.Title(language.Und, cases.NoLower, cases.Compact).String(parts[i])
	}

	example := (*T)(nil)
	typ := reflect.TypeOf(example).Elem()
	for fieldNames := parts; len(fieldNames) > 0; fieldNames = fieldNames[1:] {
		if typ.Kind() != reflect.Struct {
			return indexer, fmt.Errorf("cannot reference field %q on type %s: not a struct", fieldNames[0], typ.String())
		}
		sf, ok := typ.FieldByName(fieldNames[0])
		if !ok {
			return indexer, fmt.Errorf("unknown field %q on %s", fieldNames[0], typ.String())
		}
		indexer.index = append(indexer.index, sf.Index...)
		typ = sf.Type
	}
	return indexer, nil
}

type queryParams struct {
	Offset             int
	Limit              int
	SortField          string
	SortDescending     bool
	FilterFields       map[string][]string
	TimeRangeStart     uint64
	TimeRangeEnd       uint64
	TimeRangeOperation timeRangeRelation
	State              timeRangeState
}

func getQueryParams(r *http.Request) queryParams {
	now := uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
	qp := queryParams{
		Offset:         -1,
		Limit:          -1,
		SortField:      "identity",
		FilterFields:   make(map[string][]string),
		TimeRangeStart: now - (15 * uint64(time.Minute/time.Microsecond)),
		TimeRangeEnd:   now,
	}

	for k, v := range r.URL.Query() {
		switch k {
		case "offset":
			offset, err := strconv.Atoi(v[0])
			if err == nil {
				qp.Offset = offset
			}
		case "limit":
			limit, err := strconv.Atoi(v[0])
			if err == nil {
				qp.Limit = limit
			}
		case "sortBy":
			if v[0] != "" {
				parts := strings.Split(v[0], ".")
				if len(parts) < 2 {
					continue
				}
				switch parts[len(parts)-1] {
				case "asc":
					qp.SortDescending = false
				case "desc":
					qp.SortDescending = true
				default:
					continue
				}
				qp.SortField = strings.Join(parts[:len(parts)-1], ".")
			}
		case "timeRangeStart":
			if v[0] != "" {
				v, err := strconv.Atoi(v[0])
				if err == nil {
					qp.TimeRangeStart = uint64(v)
				}
			}
		case "timeRangeEnd":
			if v[0] != "" {
				v, err := strconv.Atoi(v[0])
				if err == nil {
					qp.TimeRangeEnd = uint64(v)
				}
			}
		case "timeRangeOperation":
			timeRangeOperation := v[0]
			switch timeRangeOperation {
			case "contains":
				qp.TimeRangeOperation = contains
			case "within":
				qp.TimeRangeOperation = within
			default:
				qp.TimeRangeOperation = intersects
			}
		case "state":
			recordState := v[0]
			switch recordState {
			case "all":
				qp.State = all
			case "active":
				qp.State = active
			case "terminated":
				qp.State = terminated
			default:
				qp.State = all
			}
		default:
			qp.FilterFields[cases.Title(language.Und, cases.NoLower).String(k)] = v
		}
	}
	return qp
}

func numInStringSlice[T int | uint64 | int64 | int32](x T, values []string) bool {
	for _, value := range values {
		i, err := strconv.ParseInt(value, 10, 64)
		if err == nil && x == T(i) {
			return true
		}
	}
	return false
}

func boolInStringSlice(cond bool, values []string) bool {

	for _, value := range values {
		switch value {
		case "true":
			if cond {
				return true
			}
		case "false":
			if !cond {
				return true
			}
		}
	}
	return false
}

func paginate(offset int, limit int, length int) (int, int) {
	start := offset
	if start < 0 {
		start = 0
	} else if start > length {
		start = length
	}
	if limit < 0 {
		limit = length
	}
	end := start + limit
	if end > length {
		end = length
	}
	return start, end
}

type timeRangeRelation int

const (
	intersects timeRangeRelation = iota
	contains
	within
)

type timeRangeState int

const (
	all timeRangeState = iota
	active
	terminated
)
