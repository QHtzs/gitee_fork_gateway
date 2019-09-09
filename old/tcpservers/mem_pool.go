package tcpservers

/*
简单内存池，及其管理器. 内存池内存只有拓开就不会回收
*/

import (
	"errors"
	"sync"
)

type MList struct {
	Next *MList
	data []byte
}

func (m *MList) New(size, num int) {
	m.data = make([]byte, size*num, size*num)
	m.Next = nil
}

func (m *MList) Add(mc *MList) {
	head := m
	for head != nil && head.Next != nil {
		head = head.Next
	}
	head.Next = mc
}

func (m *MList) Get(index int) *MList {
	head := m
	for i := 0; i < index; i++ {
		head = head.Next
	}
	return head
}

type MemHandle int

type MemCacheManage struct {
	Datas    *MList
	LEleSize int
	Num      int
	Piece    int
	Acqured  sync.Map
	Free     map[int]bool
	mutex    sync.Mutex
}

//init memcache
func (m *MemCacheManage) Init(size, num int) {
	ml := MList{}

	m.Num = num
	m.Piece = size

	ml.New(size, num)
	m.Datas = &ml

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.Free = make(map[int]bool, m.Num)
	for i := 0; i < num; i++ {
		m.Free[i] = true
	}
	m.LEleSize = 1
}

//若内存不够，则拓展
func (m *MemCacheManage) extend() int {
	ml := MList{}
	ml.New(m.Piece, m.Num)
	m.Datas.Add(&ml)
	tail := m.LEleSize * m.Num
	for i := tail; i < tail+m.Num; i++ {
		m.Free[i] = true
	}
	m.LEleSize += 1
	return tail
}

//获取空闲句柄
func (m *MemCacheManage) GetFree() MemHandle {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	v := -1
	for k, _ := range m.Free {
		v = k
		break
	}
	if v == -1 {
		v = m.extend()
	}
	return MemHandle(v)
}

//获取句柄指向内存
func (m *MemCacheManage) GetMemRef(hd MemHandle) []byte {
	src_num := int(hd) / m.Num
	m_ind := int(hd) % m.Num
	ml := m.Datas.Get(src_num)
	return ml.data[m_ind*m.Piece : m.Piece*(1+m_ind)]
}

//改变句柄状态为繁忙，需要释放ref_t次引用变成空闲
func (m *MemCacheManage) SetBusy(hd MemHandle, ref_t int) bool {
	ind := int(hd)
	res := true
	m.mutex.Lock()
	if _, ok := m.Free[ind]; ok {
		delete(m.Free, ind)
		m.Acqured.Store(ind, ref_t)
	} else {
		res = false
	}
	m.mutex.Unlock()
	return res
}

//释放一次引用
func (m *MemCacheManage) DecreaseOneRef(hd MemHandle) {
	key := int(hd)
	v, ok := m.Acqured.Load(key)
	if ok {
		ref, _ := v.(int)
		ref -= 1
		if ref <= 0 {
			m.Acqured.Delete(key)
			m.mutex.Lock()
			m.Free[key] = true
			m.mutex.Unlock()
		} else {
			m.Acqured.Store(key, ref)
		}
	}
}

//句柄
type PoolHandle struct {
	manage *MemCacheManage
	hd     MemHandle
}

//获取数据
func (p *PoolHandle) GetBytes() []byte {
	return p.manage.GetMemRef(p.hd)
}

//获取副本新的hd
func (p *PoolHandle) Brrow() PoolHandle {
	hd_ := p.manage.GetFree()
	return PoolHandle{manage: p.manage, hd: hd_}
}

//获取Hd并绑定
func (p *PoolHandle) BrrowAndAcq(ref_t int) PoolHandle {
	ok := false
	var p1 PoolHandle
	for {
		p1 = p.Brrow()
		ok = p1.Acquired(ref_t)
		if ok {
			break
		}
	}
	return p1
}

//数据拷贝到
func (p *PoolHandle) CopyTo(p1 *PoolHandle) {
	bytes0 := p.GetBytes()
	bytes1 := p1.GetBytes()
	for i := 0; i < p.manage.Piece; i++ {
		bytes1[i] = bytes0[i]
	}
}

//被引用ref_t状态
func (p *PoolHandle) Acquired(ref_t int) bool {
	return p.manage.SetBusy(p.hd, ref_t)
}

//引用减一
func (p *PoolHandle) ReleaseOnce() {
	p.manage.DecreaseOneRef(p.hd)
}

//内存池,注意:此内存池容量增大之后不会回收。也就是只会增大，不会减小
type PoolManage struct {
	Pool  map[int]*MemCacheManage
	Index []int
}

//初始化
func (p *PoolManage) Init() {
	p.Pool = make(map[int]*MemCacheManage, 10)
	p.Index = make([]int, 1, 10)
}

//添加新尺寸池, 限制size, 和 num
func (p *PoolManage) AddNew(size, num int) {
	size = size + (32-size%32)%32
	if _, ok := p.Pool[size]; !ok {
		mem := MemCacheManage{}
		mem.Init(size, num)
		p.Index = append(p.Index, size)
		p.Pool[size] = &mem
	}
}

//获取可用尺寸规格的池句柄
func (p *PoolManage) Get(scala int) (PoolHandle, error) {
	if scala%32 > 0 {
		scala = scala + (32 - scala%32)
	}
	size := scala
	sum := 1000000
	for _, v := range p.Index {
		if v-scala >= 0 && v-scala < sum {
			size = v
			sum = v - scala
		}
	}
	manage_, ok := p.Pool[size]
	if !ok {
		return PoolHandle{}, errors.New("no fit Pool")
	}
	hd_ := manage_.GetFree()
	phd := PoolHandle{
		manage: manage_,
		hd:     hd_,
	}
	return phd, nil
}
