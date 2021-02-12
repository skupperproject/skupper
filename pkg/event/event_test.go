package event

import (
	"testing"
)

func checkEventCount(t *testing.T, expected []EventCount, actual []EventCount) {
	if len(expected) != len(actual) {
		t.Errorf("Expected %d elements, got %d (%v != %v)", len(expected), len(actual), expected, actual)
	} else {
		for i, e := range expected {
			a := actual[i]
			if e.Key != a.Key {
				t.Errorf("Wrong key, expected %s, got %s: %v", e.Key, a.Key, a)
			}
			if e.Count != a.Count {
				t.Errorf("Wrong count, expected %d, got %d: %v", e.Count, a.Count, a)
			}
		}
	}

}

func checkEventGroup(t *testing.T, expected []EventGroup, actual []EventGroup) {
	if len(expected) != len(actual) {
		t.Errorf("Expected %d elements, got %d (%v != %v)", len(expected), len(actual), expected, actual)
	} else {
		for i, a := range actual {
			e := expected[i]
			if e.Name != a.Name {
				t.Errorf("Wrong name, expected %s, got %s", e.Name, a.Name)
			}
			if e.Total != a.Total {
				t.Errorf("Wrong total, expected %d, got %d", e.Total, a.Total)
			}
			checkEventCount(t, e.Counts, a.Counts)
		}
	}

}

func TestEventStore(t *testing.T) {
	stopper := make(chan struct{})
	StartDefaultEventStore(stopper)
	Record("foo", "foo, bar and baz!")
	expected := []EventGroup {
		{
			Name: "foo",
			Total: 1,
			Counts: []EventCount {
				{
					Key: "foo, bar and baz!",
					Count: 1,
				},
			},
		},
	}
	actual := Query()
	checkEventGroup(t, expected, actual)

	Record("foo", "something else")
	Recordf("foo", "%s, %s and %s!", "foo", "bar", "baz")
	Record("whatsit", "blah")
	expected = []EventGroup {
		{
			Name: "whatsit",
			Total: 1,
			Counts: []EventCount {
				{
					Key: "blah",
					Count: 1,
				},
			},
		},
		{
			Name: "foo",
			Total: 3,
			Counts: []EventCount {
				{
					Key: "foo, bar and baz!",
					Count: 2,
				},
				{
					Key: "something else",
					Count: 1,
				},
			},
		},
	}
	actual = Query()
	checkEventGroup(t, expected, actual)

	Record("foo", "one")
	Record("foo", "two")
	Record("foo", "three")
	Record("foo", "four")
	expected = []EventGroup {
		{
			Name: "foo",
			Total: 7,
			Counts: []EventCount {
				{
					Key: "four",
					Count: 1,
				},
				{
					Key: "three",
					Count: 1,
				},
				{
					Key: "two",
					Count: 1,
				},
				{
					Key: "one",
					Count: 1,
				},
				{
					Key: "foo, bar and baz!",
					Count: 2,
				},
			},
		},
		{
			Name: "whatsit",
			Total: 1,
			Counts: []EventCount {
				{
					Key: "blah",
					Count: 1,
				},
			},
		},
	}
	actual = Query()
	checkEventGroup(t, expected, actual)
	close(stopper)
}
