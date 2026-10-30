package pool

import (
	"sync"
)

// Resetter описывает типы, которые умеют сбрасывать состояние
type Resetter interface {
	comparable
	Reset()
}

// Pool — типобезопасная обёртка над sync.Pool: тип объектов проверяется при компиляции.
// При возврате в пул объект автоматически сбрасывается через Reset.
type Pool[T Resetter] struct {
	pool sync.Pool
}

// New создаёт пул объектов типа T.
// Функция create вызывается, когда пул пуст и нужен новый объект.
func New[T Resetter](create func() T) *Pool[T] {
	pool := &Pool[T]{
		pool: sync.Pool{
			New: func() any { return create() },
		},
	}
	return pool
}

// Get возвращает объект из пула. Если пул пуст, создаёт новый через create.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put сбрасывает объект через Reset и возвращает его в пул.
// Put проверяет нулевое значение T сам: для указателя это nil, поэтому вызывающему коду
// не нужна проверка перед Put, а реализациям Reset не нужно учитывать nil-ресивер.
// Такое значение игнорируется: Reset не вызывается, в пул ничего не попадает.
func (p *Pool[T]) Put(x T) {
	var zero T
	if x == zero {
		return
	}
	x.Reset()
	p.pool.Put(x)
}
