package main

import (
	"container/list"
	"sync"
	"time"
)

type QueueHandle struct {
	DeQueue *list.List
	NotiFy  chan int
	MaxSize int
	mutex   sync.Mutex
	status  bool
}

func (q *QueueHandle) Init(qsize int) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.status {
		return
	}
	if qsize > 0 {
		q.NotiFy = make(chan int, qsize)
	} else {
		q.NotiFy = make(chan int, 1)
	}
	q.DeQueue = list.New()
	q.MaxSize = qsize
	q.status = true
}

func (q *QueueHandle) QSize() int {
	return q.DeQueue.Len()
}

func (q *QueueHandle) AddData(v interface{}) {
	if q.MaxSize > 0 {
		q.NotiFy <- 1
	} else {
		select {
		case q.NotiFy <- 1:
			break
		default:
			break
		}
	}
	q.mutex.Lock()
	q.DeQueue.PushBack(v)
	q.mutex.Unlock()
}

func (q *QueueHandle) PopData() interface{} {
	if q.MaxSize > 0 || q.DeQueue.Len() <= 0 {
		<-q.NotiFy
	}

	q.mutex.Lock()
	var ret interface{} = nil
	if q.DeQueue.Len() > 0 {
		ret = q.DeQueue.Remove(q.DeQueue.Front())
	}
	q.mutex.Unlock()
	return ret
}

type FifoQueue struct {
	mutex  sync.Mutex
	deque  map[string]QueueHandle
	single QueueHandle
	inited bool
}

func (f *FifoQueue) Put(serial string, v interface{}) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if !f.inited {
		f.inited = true
		f.deque = make(map[string]QueueHandle, 1)
	}
	v_, ok := f.deque[serial]
	if ok {
		v_.AddData(v)
	} else {
		v_ = QueueHandle{status: false}
		v_.Init(0)
		v_.AddData(v)
		f.deque[serial] = v_
	}

}

func (f *FifoQueue) Get(serial string) interface{} {
	f.mutex.Lock()
	v_, ok := f.deque[serial]
	f.mutex.Unlock()
	var v interface{} = nil
	if ok {
		v = v_.PopData()
	} else {
		time.Sleep(2 * time.Second)
	}
	return v
}

func (f *FifoQueue) SingelPut(v interface{}) {
	if !f.single.status {
		f.single.Init(0)
	}
	f.single.AddData(v)

}

func (f *FifoQueue) SingleGet() interface{} {
	if !f.single.status {
		f.single.Init(0)
	}
	return f.single.PopData()

}
