package main

import (
	"container/list"
	"sync"
)

type FifoQueue struct {
	mutex  sync.Mutex
	dqueue map[string]*list.List
	size   int
}

func (f *FifoQueue) Put(serial string, v interface{}) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.size == 0 { //golang int 不显式赋值则默认为0。
		f.dqueue = make(map[string]*list.List, 1)
		f.size = 1
	}
	l, ok := f.dqueue[serial]
	if ok {
		l.PushBack(v)
	} else {
		l = list.New()
		l.PushBack(v)
		f.dqueue[serial] = l
	}
}

func (f *FifoQueue) Get(serial string) interface{} {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.size == 0 { //golang int 不显式赋值则默认为0。
		f.dqueue = make(map[string]*list.List, 1)
		f.size = 1
	}
	l, ok := f.dqueue[serial]
	if ok {
		if l.Len() <= 0 {
			return nil
		} else {
			return l.Remove(l.Front())
		}
	}
	return nil
}
