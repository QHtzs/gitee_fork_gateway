package main

/*
简单内存池，及其管理器. 内存池内存不会回收
*/

import (
	"sync"
	"sync/atomic"
)

//内存申请
type MemBrick struct {
	NextBrick       *MemBrick  //下一个逻辑相连的内存块
	AllocBytes      []byte     //申请的大块内存
	AllocPieceIndex []bool     //内存是否被占据
	EachSize        int        //每块的大小
	mutex           sync.Mutex //
}

//内存申请 piece内存含多少块， each_size每块的size:建议为4的倍数
func (m *MemBrick) Alloc(piece, each_size int) {
	m.NextBrick = nil
	m.AllocBytes = make([]byte, piece*each_size, piece*each_size)
	m.AllocPieceIndex = make([]bool, piece, piece)
	for i := 0; i < piece; i++ {
		m.AllocPieceIndex[i] = true
	}
	m.EachSize = each_size
}

//内存块逻辑相连
func (m *MemBrick) Connect(mb *MemBrick) {
	ptr := m
	for ptr != nil && ptr.NextBrick != nil {
		ptr = ptr.NextBrick
	}
	ptr.NextBrick = mb
}

//检查申请可用内存index
func (m *MemBrick) CheckAndAcquireFree() int {
	ret := -1
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for index, status := range m.AllocPieceIndex {
		if status {
			ret = index
			break
		}
	}
	if ret >= 0 {
		m.AllocPieceIndex[ret] = false
	}
	return ret
}

//释放占有权
func (m *MemBrick) ReleaseAcq(index int) {
	if index < 0 {
		return
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.AllocPieceIndex[index] = true
}

//获取内存
func (m *MemBrick) LoadMem(index int) ([]byte, int) {
	return m.AllocBytes[index*m.EachSize : index*m.EachSize+m.EachSize], m.EachSize
}

type MemEntity struct {
	Index    int
	AckTimes *int32
	Brick    *MemBrick
	PoolPtr  *MemPool
}

func (m *MemEntity) Bytes() ([]byte, int) {
	return m.Brick.LoadMem(m.Index)
}

func (m *MemEntity) Copy(ack_time, cp_len int) *MemEntity {
	src, size := m.Bytes()
	entity := m.PoolPtr.GetEntity(ack_time, size)
	dst, _ := entity.Bytes()
	for i := 0; i < cp_len; i++ {
		dst[i] = src[i]
	}
	return entity
}

func (m *MemEntity) ReleaseOnece() {
	v := atomic.AddInt32(m.AckTimes, -1)
	if v == 0 {
		m.Brick.ReleaseAcq(m.Index)
	}
}

func (m *MemEntity) FullRelease() {
	m.Brick.ReleaseAcq(m.Index)
}

type MemPool struct {
	MemBrickList *MemBrick
}

func (m *MemPool) findBestFix(size int) int {
	ret := 4
	for ret < size {
		ret *= 2
	}
	return ret
}

func (m *MemPool) addNew(size int) {
	mc := new(MemBrick)
	mc.Alloc(1024, size)
	if m.MemBrickList == nil {
		m.MemBrickList = mc
	} else {
		m.MemBrickList.Connect(mc)
	}
}

func (m *MemPool) getEntity(ack_time, size int) *MemEntity {
	if m.MemBrickList == nil {
		fit := m.findBestFix(size)
		m.addNew(fit)
		return nil
	}

	ptr := m.MemBrickList
	index := -1

	for ptr != nil {
		if ptr.EachSize >= size {
			index = ptr.CheckAndAcquireFree()
			if index >= 0 {
				break
			}
		}
		ptr = ptr.NextBrick
	}

	if ptr == nil || index == -1 {
		fit := m.findBestFix(size)
		m.addNew(fit)
		return nil
	}

	ret := new(MemEntity)
	ret.Index = index
	ret.AckTimes = new(int32)
	*ret.AckTimes = int32(ack_time)
	ret.Brick = ptr
	ret.PoolPtr = m
	return ret
}

func (m *MemPool) GetEntity(ack_time, size int) *MemEntity {
	ret := m.getEntity(ack_time, size)
	if ret == nil {
		ret = m.getEntity(ack_time, size)
	}
	return ret
}
