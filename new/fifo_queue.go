package main

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
)

//channel形成的goroutine不会释放，直到程序结束
type QueueHandle struct {
	DeQueue        *list.List
	NotiFy         chan int
	LastActiveTime int64
	mutex          sync.Mutex
	status         bool
	free           bool
}

func (q *QueueHandle) LazyInit() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.LastActiveTime = time.Now().Unix()
	if q.status {
		return
	}
	q.NotiFy = make(chan int, 1)
	q.DeQueue = list.New()
	q.status = true
	q.free = false
}

func (q *QueueHandle) Size() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.DeQueue.Len()
}

func (q *QueueHandle) AddData(v interface{}) bool {
	q.LazyInit()

	select {
	case q.NotiFy <- 1:
		break
	default:
		break
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.free {
		return false
	} else {
		q.DeQueue.PushBack(v)
		return true
	}
}

func (q *QueueHandle) PopData() interface{} {
	q.LazyInit()

	if q.Size() <= 0 {
		<-q.NotiFy
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.DeQueue.Len() > 0 {
		return q.DeQueue.Remove(q.DeQueue.Front())
	}
	return nil
}

func (q *QueueHandle) CanGc() bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	b1 := time.Now().Unix()-q.LastActiveTime > 3000
	return q.status && b1 && q.Size() <= 0
}

func (q *QueueHandle) SetFree() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	//close(q.NotiFy) 不关闭也能被回收
	q.free = true
}

type FifoQueue struct {
	deque    sync.Map
	single   *QueueHandle
	calltime uint64
}

func (f *FifoQueue) Put(serial string, v interface{}) {

	actual, ok := f.deque.LoadOrStore(serial, &QueueHandle{})
	if ok {
		mp, ok := actual.(*QueueHandle)
		if ok {
			if mp.AddData(v) {
				return
			} else {
				time.Sleep(50 * time.Millisecond)
				f.Put(serial, v) //递归
			}
		}
	}

	f.deque.Range(func(key interface{}, value interface{}) bool {
		sk, _ := key.(string)
		if sk == serial {
			act_v, ok := value.(*QueueHandle)
			if ok && act_v != nil {
				act_v.AddData(v)
			}
			return false
		} else {
			return true
		}
	})

}

func (f *FifoQueue) Get(serial string) interface{} {

	atomic.AddUint64(&f.calltime, 1)

	if atomic.LoadUint64(&f.calltime)%1000 == 0 {
		capture := make(map[string]bool, 1)
		f.deque.Range(func(key interface{}, value interface{}) bool {
			name, _ := key.(string)
			act_v, ok := value.(*QueueHandle)
			if ok {
				if act_v.CanGc() {
					capture[name] = true
					act_v.SetFree()
				}
			}
			return true
		})

		for nk, _ := range capture {
			if nk != "" {
				f.deque.Delete(nk)
			}
		}
	}

	actual, ok := f.deque.Load(serial)
	if ok {
		q, ok := actual.(*QueueHandle)
		if ok {
			return q.PopData()
		}
	}
	time.Sleep(200 * time.Millisecond)
	return nil
}

func (f *FifoQueue) SingelPut(v interface{}) {
	if f.single == nil {
		f.single = &QueueHandle{}
	}
	f.single.AddData(v)
}

func (f *FifoQueue) SingleGet() interface{} {
	if f.single == nil {
		f.single = &QueueHandle{}
	}
	return f.single.PopData()
}
