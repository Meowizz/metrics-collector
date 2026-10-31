package pool

import (
	"testing"
)

type testObject struct {
	count int
}

func (o *testObject) Reset() {
	o.count = 0
}

func TestPool(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{}
	})

	obj := p.Get()
	obj.count = 42

	p.Put(obj)

	clean := p.Get()

	if clean.count != 0 {
		t.Errorf("ожидали count = 0, получили %d", clean.count)
	}
}

func TestPutNilPointer(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{}
	})

	p.Put(nil)

	if got := p.Get(); got == nil {
		t.Error("Get вернул nil, ожидали объект из create")
	}
}
