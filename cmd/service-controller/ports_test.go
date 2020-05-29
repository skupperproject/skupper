package main

import (
	"testing"
)

func TestPortAllocationCase1(t *testing.T) {
	ports := newFreePorts()
	ports.inuse(1024)
	ports.inuse(1025)
	ports.inuse(1027)
	ports.inuse(1028)
	if next, err := ports.nextFreePort(); err != nil || next != 1026 {
		t.Errorf(`Expected 1026 as first free port, got %d`, next)
	}
	if next, err := ports.nextFreePort(); err != nil || next != 1029 {
		t.Errorf(`Expected 1029 as second free port, got %d`, next)
	}
}

func TestPortAllocationCase2(t *testing.T) {
	ports := newFreePorts()
	ports.inuse(1026)
	ports.inuse(1025)
	if next, err := ports.nextFreePort(); err != nil || next != 1024 {
		t.Errorf(`Expected 1024 as first free port, got %d`, next)
	}
	if next, err := ports.nextFreePort(); err != nil || next != 1027 {
		t.Errorf(`Expected 1027 as first free port, got %d`, next)
	}
	if ports.inuse(1026) == true {
		t.Errorf(`inuse(1026) == true, should be false`)
	}
}

func TestPortAllocationCase3(t *testing.T) {
	ports := newFreePorts()
	ports.inuse(1027)
	if ports.String() != "[(1024-1026), (1028-65535)]" {
		t.Errorf(`Invalid internal state after consuming 1027: %s`, ports)
	}
	ports.inuse(1025)
	if ports.String() != "[(1024-1024), (1026-1026), (1028-65535)]" {
		t.Errorf(`Invalid internal state after consuming 1025: %s`, ports)
	}
	ports.inuse(1024)
	if ports.String() != "[(1026-1026), (1028-65535)]" {
		t.Errorf(`Invalid internal state after consuming 1024: %s`, ports)
	}
	ports.inuse(1026)
	if ports.String() != "[(1028-65535)]" {
		t.Errorf(`Invalid internal state after consuming 1026: %s`, ports)
	}
}

func TestPortAllocationCase4(t *testing.T) {
	ports := newFreePorts()
	ports.inuse(1027)
	if ports.String() != "[(1024-1026), (1028-65535)]" {
		t.Errorf(`Invalid internal state after consuming 1027: %s`, ports)
	}
	ports.inuse(2025)
	if ports.String() != "[(1024-1026), (1028-2024), (2026-65535)]" {
		t.Errorf(`Invalid internal state after consuming 2025: %s`, ports)
	}
	ports.inuse(1500)
	if ports.String() != "[(1024-1026), (1028-1499), (1501-2024), (2026-65535)]" {
		t.Errorf(`Invalid internal state after consuming 1024: %s`, ports)
	}
	for i := 1028; i <= 1499; i++ {
		ports.inuse(i)
	}
	if ports.String() != "[(1024-1026), (1501-2024), (2026-65535)]" {
		t.Errorf(`Invalid internal state after consuming 1028-1499: %s`, ports)
	}
}

func TestPortAllocationFailure(t *testing.T) {
	ports := newFreePorts()
	for i := MIN_PORT; i <= MAX_PORT; i++ {
		ports.inuse(i)
	}
	if next, err := ports.nextFreePort(); err == nil || next != 0 {
		t.Errorf(`Expected allocation failure, got %d`, next)
	}
}

func TestPortReleaseCase1(t *testing.T) {
	ports := newFreePorts()
	for i := MIN_PORT; i <= MAX_PORT; i++ {
		ports.inuse(i)
	}
	ports.release(20000)
	if len(ports.Available) != 1 {
		t.Errorf(`After releasing 20000 expected one available range, got: %d`, len(ports.Available))
	}
	if len(ports.Available) > 1 && ports.Available[0].size() != 1 {
		t.Errorf(`After releasing 20000 expected range to be of size 1, got: %d`, ports.Available[0].size())
	}
	ports.release(20002)
	if len(ports.Available) != 2 {
		t.Errorf(`After releasing 20002 expected two available ranges, got: %d`, len(ports.Available))
	}
	ports.release(20001)
	if len(ports.Available) != 1 {
		t.Errorf(`After releasing 20001 expected one available range, got: %d`, len(ports.Available))
	}
	if len(ports.Available) > 1 && ports.Available[0].size() != 3 {
		t.Errorf(`After releasing 20001 expected range to be of size 3, got: %d`, ports.Available[0].size())
	}
	if ports.release(20001) == true {
		t.Errorf(`release(20001) == true, should be false`)
	}
}

func TestPortReleaseCase2(t *testing.T) {
	ports := newFreePorts()
	for i := MIN_PORT; i <= MAX_PORT; i++ {
		ports.inuse(i)
	}
	ports.release(1027)
	if ports.String() != "[(1027-1027)]" {
		t.Errorf(`Invalid internal state after releasing 1027: %s`, ports)
	}
	ports.release(1025)
	if ports.String() != "[(1025-1025), (1027-1027)]" {
		t.Errorf(`Invalid internal state after releasing 1025: %s`, ports)
	}
	ports.release(1024)
	if ports.String() != "[(1024-1025), (1027-1027)]" {
		t.Errorf(`Invalid internal state after releasing 1024: %s`, ports)
	}
	ports.release(1026)
	if ports.String() != "[(1024-1027)]" {
		t.Errorf(`Invalid internal state after releasing 1026: %s`, ports)
	}
}

func TestMergePortRange1(t *testing.T) {
	a := PortRange {
		Start: 10,
		End: 11,
	}
	b := PortRange {
		Start: 8,
		End: 15,
	}
	if b.merge(a) {
		if b.Start != 8 && b.End != 15 {
			t.Errorf(`expected %s, got %s`, b, PortRange {
				Start: 8,
				End: 15,
			})
		}
	} else {
		t.Errorf(`merge did not succeed`)
	}
}

func TestMergePortRange2(t *testing.T) {
	a := PortRange {
		Start: 10,
		End: 11,
	}
	b := PortRange {
		Start: 8,
		End: 15,
	}
	if a.merge(b) {
		if a.Start != b.Start && a.End != b.End {
			t.Errorf(`expected %s, got %s`, b, a)
		}
	} else {
		t.Errorf(`merge did not succeed`)
	}
}

func TestMergePortRange3(t *testing.T) {
	a := PortRange {
		Start: 10,
		End: 11,
	}
	b := PortRange {
		Start: 8,
		End: 9,
	}
	if a.merge(b) {
		if a.Start != 8 && a.End != 11 {
			t.Errorf(`expected %s, got %s`, b, PortRange {
				Start: 8,
				End: 11,
			})
		}
	} else {
		t.Errorf(`merge did not succeed`)
	}
}

func TestMergePortRange4(t *testing.T) {
	a := PortRange {
		Start: 10,
		End: 15,
	}
	b := PortRange {
		Start: 12,
		End: 19,
	}
	expected := PortRange {
		Start: 10,
		End: 19,
	}
	if a.merge(b) {
		if a.Start != expected.Start && a.End != expected.End {
			t.Errorf(`expected %s, got %s`, b, expected)
		}
	} else {
		t.Errorf(`merge did not succeed`)
	}
}

func TestMergePortRange5(t *testing.T) {
	a := PortRange {
		Start: 20,
		End: 25,
	}
	b := PortRange {
		Start: 12,
		End: 21,
	}
	expected := PortRange {
		Start: 12,
		End: 25,
	}
	if a.merge(b) {
		if a.Start != expected.Start && a.End != expected.End {
			t.Errorf(`expected %s, got %s`, b, expected)
		}
	} else {
		t.Errorf(`merge did not succeed`)
	}
}

func TestMergePortRangeFailed(t *testing.T) {
	a := PortRange {
		Start: 20,
		End: 25,
	}
	b := PortRange {
		Start: 12,
		End: 14,
	}
	if a.merge(b) {
		t.Errorf(`merge should not succeed`)
	}
}
