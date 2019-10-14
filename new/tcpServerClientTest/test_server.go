// test_server
package main

/*
测试文件
请单独编译
*/

import (
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

var CONNECTTING_NUM int64 = 0
var FAILED_CONNECT_NUM int64 = 0
var LOST_CONNECT int64 = 0

func swap(a, b *byte) {
	c := *a
	*a = *b
	*b = c
}

func MixFour(src []byte, length int) {
	i := 0
	for i < length-3 {
		swap(&src[i+2], &src[i+3])
		swap(&src[i], &src[i+2])
		i += 4
	}
}

func DeMixFour(src []byte, length int) {
	i := 0
	for i < length-3 {
		swap(&src[i], &src[i+2])
		swap(&src[i+2], &src[i+3])
		i += 4
	}
}

func EncryPt(src, dst []byte, length int) int { // length % 3 === 0
	base64.StdEncoding.Encode(dst, src[0:length])
	length = length / 3
	length *= 4
	MixFour(dst, length)
	return length
}

func DeCrypt(src, dst []byte, length int) int { // length % 4 === 0
	DeMixFour(src, length)
	length, _ = base64.StdEncoding.Decode(dst, src[0:length])
	//对面可能传来的是含 ==字符串，即不为3倍数的字符串加密,故以返回长度为准
	j := 0
	for i := 0; i+j < length; i++ {
		if j > 0 {
			dst[i] = dst[i+j]
		}

		if dst[i] == 0 || dst[i] == 32 {
			j += 1
		}
	}
	length -= j
	return length
}

type TcpClientTest struct {
	ID        string
	Socket    net.Conn
	Address   string
	Beat      []byte
	Freq      int64
	NeedEbcry bool
	Buff      []byte
	Buff2     []byte
}

func (t *TcpClientTest) Connect() bool {
	tcpAddr, _ := net.ResolveTCPAddr("tcp", t.Address)
	con, err := net.DialTCP("tcp", nil, tcpAddr)
	t.Socket = con
	fmt.Println("error:", err)
	return err == nil
}

func (t *TcpClientTest) Write(data []byte) bool {
	var err error = nil
	ret := true
	if t.NeedEbcry {
		tmp := make([]byte, 2*len(data))
		ln := EncryPt(data, tmp, len(data))
		_, err = t.Socket.Write(tmp[0:ln])
	} else {
		_, err = t.Socket.Write(data)
	}
	if err != nil {
		ret = false
	}
	return ret
}

func (t *TcpClientTest) CreateBeat() {
	if t.Freq > 0 {
		for {
			time.Sleep(time.Duration(t.Freq) * time.Second)
			t.Write(t.Beat)
		}
	}
}

func (t *TcpClientTest) Read() (string, bool) {
	size, err := t.Socket.Read(t.Buff)
	if err != nil {
		atomic.AddInt64(&LOST_CONNECT, 1)
		atomic.AddInt64(&CONNECTTING_NUM, -1)
		t.Socket.Close()
		return "", false
	}
	if t.NeedEbcry {
		length := DeCrypt(t.Buff, t.Buff2, size)
		return string(t.Buff2[0:length]), true
	} else {
		return string(t.Buff[0:size]), true
	}
}

func (t *TcpClientTest) Start(bbb bool) {
	bl := t.Connect()
	if bl {
		atomic.AddInt64(&CONNECTTING_NUM, 1)
		t.Write([]byte(t.ID))
		acp, _ := t.Read()
		fmt.Println(acp)

		go t.CreateBeat()

		go func(tt *TcpClientTest) {
			for {
				str, ok := tt.Read()
				fmt.Println(tt.ID, ":", str, "current con:", atomic.LoadInt64(&CONNECTTING_NUM),
					" failed:", atomic.LoadInt64(&FAILED_CONNECT_NUM),
					" lost connect", atomic.LoadInt64(&LOST_CONNECT))
				if !ok {
					break
				}
			}
		}(t)

		if bbb {
			go func(tt *TcpClientTest) {
				for {
					wstr := fmt.Sprintf(`{"time":"%d", "id":"%s", "a": {"b":"c"}}`, time.Now().Unix(), tt.ID)
					lft := len(wstr) % 3
					if lft == 1 {
						wstr += "    "
					} else if lft == 2 {
						wstr += " "
					}
					ok := tt.Write([]byte(wstr))
					time.Sleep(20 * time.Second)
					if !ok {
						break
					}
				}
			}(t)
		}

	} else {
		atomic.AddInt64(&FAILED_CONNECT_NUM, 1)
	}
}

func CreateTcps(id string) {
	app := TcpClientTest{
		ID:      id,
		Address: ":9000",
		Freq:    0,
		Buff:    make([]byte, 512),
	}

	web := TcpClientTest{
		ID:      id,
		Address: ":8001",
		Freq:    0,
		Buff:    make([]byte, 512),
	}

	gateway := TcpClientTest{
		ID:        id,
		Address:   ":8000",
		Freq:      0,
		NeedEbcry: true,
		Beat:      []byte("ALIVE"),
		Buff:      make([]byte, 512),
		Buff2:     make([]byte, 1024),
	}

	app.Start(false)
	web.Start(false)
	gateway.Start(true)
}

func main() {
	for i := 0; i < 1000; i++ { // num * 3
		id := "TTG_DEMO_" + strconv.Itoa(i)
		k := len(id) % 3
		if k > 0 {
			for j := 0; j < 3-k; j++ {
				id += " "
			}
		}
		go CreateTcps(id)
	}
	time.Sleep(1 * time.Hour)
}
