package main

/*
不追求性能极致，故采用一般锁。而不模拟sync.Map结构

*/

import (
	"sync"
)

type NetConMap struct {
	Deque    sync.Map
	AllowDup bool
	mtx      sync.Mutex
}

func (n *NetConMap) SetAllowDup(bl bool) {
	n.AllowDup = bl
}

func (n *NetConMap) Delete(serial, subkey string) {
	if n.AllowDup {
		v, ok := n.Deque.Load(serial)
		if ok {
			mp, ok := v.(*sync.Map)
			if ok { //
				mp.Delete(subkey)
			}
		}
	} else {
		n.Deque.Delete(serial)
	}
}

func (n *NetConMap) Load(serial, subkey string) (value interface{}, ok bool) {
	if n.AllowDup {
		v, ok := n.Deque.Load(serial)
		if ok {
			mp, ok := v.(*sync.Map)
			if ok {
				return mp.Load(subkey)
			}
		}
		return nil, false
	} else {
		return n.Deque.Load(serial)
	}
}

func (n *NetConMap) LoadOrStore(serial, subkey string, value interface{}) (actual interface{}, ok bool) {
	if n.AllowDup {
		v, ok := n.Deque.Load(serial)
		if ok {
			mp, ok := v.(*sync.Map)
			if ok {
				return mp.LoadOrStore(subkey, value)
			}
		}

		n.mtx.Lock()
		v, ok = n.Deque.Load(serial)
		if ok {
			mp, ok := v.(*sync.Map)
			if ok {
				return mp.LoadOrStore(subkey, value)
			}
		}
		mp := new(sync.Map)
		*mp = sync.Map{}
		n.Deque.Store(serial, mp)
		n.mtx.Unlock()

		return mp.LoadOrStore(subkey, value)
	} else {
		return n.Deque.LoadOrStore(serial, value)
	}
}

func (n *NetConMap) Range(serial string, f func(key interface{}, value interface{}) bool) {
	if n.AllowDup {
		v, ok := n.Deque.Load(serial)
		if ok {
			mp, ok := v.(*sync.Map)
			if ok {
				mp.Range(f)
			}
		}
	} else {
		n.Deque.Range(f)
	}
}

func (n *NetConMap) Store(serial, subkey string, value interface{}) {
	if n.AllowDup {
		v, ok := n.Deque.Load(serial)
		if ok {
			mp, ok := v.(*sync.Map)
			if ok {
				mp.Store(subkey, value)
				return //break func
			}
		}

		n.mtx.Lock()
		v, ok = n.Deque.Load(serial)
		if ok {
			mp, ok := v.(*sync.Map)
			if ok {
				mp.Store(subkey, value)
				return //break func
			}
		}
		mp := new(sync.Map)
		*mp = sync.Map{}
		n.Deque.Store(serial, mp)
		n.mtx.Unlock()

		mp.Store(subkey, value)

	} else {
		n.Deque.Store(serial, value)
	}
}

func (n *NetConMap) IsKeyExist(serial string) bool {
	if n.AllowDup {
		v, ok := n.Deque.Load(serial)
		if ok {
			mp, ok := v.(*sync.Map)
			if ok {
				lambda_catch := false
				mp.Range(func(key interface{}, value interface{}) bool {
					lambda_catch = true
					return false
				})
				return lambda_catch
			}
		}
		return false
	} else {
		_, ok := n.Deque.Load(serial)
		return ok
	}
}
