package collector

import "github.com/skupperproject/skupper/pkg/vanflow"

type changeEvent interface {
	ID() string
	GetTypeMeta() vanflow.TypeMeta
}

type addEvent struct {
	Record vanflow.Record
}

func (i addEvent) ID() string                    { return i.Record.Identity() }
func (i addEvent) GetTypeMeta() vanflow.TypeMeta { return i.Record.GetTypeMeta() }

type deleteEvent struct {
	Record vanflow.Record
}

func (i deleteEvent) ID() string                    { return i.Record.Identity() }
func (i deleteEvent) GetTypeMeta() vanflow.TypeMeta { return i.Record.GetTypeMeta() }

type updateEvent struct {
	Prev vanflow.Record
	Curr vanflow.Record
}

func (i updateEvent) ID() string                    { return i.Curr.Identity() }
func (i updateEvent) GetTypeMeta() vanflow.TypeMeta { return i.Curr.GetTypeMeta() }
