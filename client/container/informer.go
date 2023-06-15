package container

type Informer[T any] interface {
	OnAdd(obj T)
	OnUpdate(oldObj, newObj T)
	OnDelete(obj T)
}

type InformerBase[T any] struct {
	Add    func(obj T)
	Update func(oldObj, newObj T)
	Delete func(obj T)
}

func (e *InformerBase[T]) OnAdd(obj T) {
	e.Add(obj)
}

func (e *InformerBase[T]) OnUpdate(oldObj, newObj T) {
	e.Update(oldObj, newObj)
}

func (e *InformerBase[T]) OnDelete(obj T) {
	e.Delete(obj)
}
