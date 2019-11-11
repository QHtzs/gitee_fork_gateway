// test_server
package main

/*
测试文件
请单独编译
*/

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"sync/atomic"
	"time"
)

var CONNECTTING_NUM int64 = 0
var FAILED_CONNECT_NUM int64 = 0
var LOST_CONNECT int64 = 0

type Configure struct {
	XMLName    xml.Name `xml:"Config"`
	Wp         string   `xml:"Wp"`
	Gp         string   `xml:"Gp"`
	Cp         string   `xml:"Cp"`
	Ws         bool     `xml:"Ws"`
	Gs         bool     `xml:"Gs"`
	Cs         bool     `xml:"Cs"`
	We         bool     `xml:"We"`
	Ge         bool     `xml:"Ge"`
	Ce         bool     `xml:"Ce"`
	Wn         int      `xml:"Wn"`
	Gn         int      `xml:"Gn"`
	Cn         int      `xml:"Cn"`
	Wf         int      `xml:"Wf"`
	Gf         int      `xml:"Gf"`
	Cf         int      `xml:"Cf"`
	Wmsg       bool     `xml:"Wmsg"`
	Gmsg       bool     `xml:"Gmsg"`
	Cmsg       bool     `xml:"Cmsg"`
	ConfirmFmt string   `xml:"ConfirmFmt"`
	Inteval    int64    `xml:"Inteval"`
}

func (c *Configure) Init(xmlfile string) {
	file, err := os.Open(xmlfile)
	if err != nil {
		panic("配置文件读取失败，请确保配置文件存在且路径正确:" + err.Error())
	}
	defer file.Close()
	xml_bytes, err := ioutil.ReadAll(file)
	if err != nil {
		panic("配置文件读取失败，请确保配置文件存在且路径正确:" + err.Error())
	}
	err = xml.Unmarshal(xml_bytes, c)
}

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
	TypeName  string
	ID        string
	Socket    net.Conn
	Address   string
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
		//fmt.Println("--", t.Buff, "--", t.Buff2)
		return string(t.Buff2[0:length]), true
	} else {
		return string(t.Buff[0:size]), true
	}
}

func (t *TcpClientTest) Start(bbb bool, cfg *Configure) {
	bl := t.Connect()
	if bl {
		atomic.AddInt64(&CONNECTTING_NUM, 1)
		t.Write([]byte(t.ID))
		acp, _ := t.Read()
		fmt.Println(acp)

		go func(tt *TcpClientTest) {
			for {
				str, ok := tt.Read()
				fmt.Println(tt.TypeName, tt.ID, ":", str, "current con:", atomic.LoadInt64(&CONNECTTING_NUM),
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
					wstr := fmt.Sprintf(`{"time":"%d","Type":"%s", "id":"%s", "a": {"b":"c"}}`, time.Now().Unix(), tt.TypeName, tt.ID)
					lft := len(wstr) % 3
					if lft == 1 {
						wstr += "    "
					} else if lft == 2 {
						wstr += " "
					}
					ok := tt.Write([]byte(wstr))
					time.Sleep(time.Duration(cfg.Inteval) * time.Second)
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

func CreateTcps(id string, c *Configure, tp string) {
	if tp == "C" {
		app := TcpClientTest{
			TypeName:  "App",
			ID:        id,
			Address:   ":" + c.Cp,
			NeedEbcry: c.Ce,
			Buff:      make([]byte, 512),
			Buff2:     make([]byte, 1024),
		}

		if c.Cs {
			app.Start(c.Cmsg, c)
		}
	}

	if tp == "W" {

		web := TcpClientTest{
			TypeName:  "Web",
			ID:        id,
			Address:   ":" + c.Wp,
			NeedEbcry: c.We,
			Buff:      make([]byte, 512),
			Buff2:     make([]byte, 1024),
		}

		if c.Ws {
			web.Start(c.Wmsg, c)
		}
	}

	if tp == "G" {
		gateway := TcpClientTest{
			TypeName:  "GateWay",
			ID:        id,
			Address:   ":" + c.Gp,
			NeedEbcry: c.Ge,
			Buff:      make([]byte, 512),
			Buff2:     make([]byte, 1024),
		}

		if c.Gs {
			gateway.Start(c.Gmsg, c)
		}
	}
}

func main() {
	cfg := Configure{}
	cfg.Init("test_conf.xml")
	for i := cfg.Wf; i < cfg.Wn+cfg.Wf; i++ {
		id := fmt.Sprintf(cfg.ConfirmFmt, i)
		k := len(id) % 3
		if k > 0 {
			for j := 0; j < 3-k; j++ {
				id += " "
			}
		}
		go CreateTcps(id, &cfg, "W")
	}

	for i := cfg.Gf; i < cfg.Gn+cfg.Gf; i++ {
		id := fmt.Sprintf(cfg.ConfirmFmt, i)
		k := len(id) % 3
		if k > 0 {
			for j := 0; j < 3-k; j++ {
				id += " "
			}
		}
		go CreateTcps(id, &cfg, "G")
	}

	for i := cfg.Cf; i < cfg.Cn+cfg.Cf; i++ {
		id := fmt.Sprintf(cfg.ConfirmFmt, i)
		k := len(id) % 3
		if k > 0 {
			for j := 0; j < 3-k; j++ {
				id += " "
			}
		}
		go CreateTcps(id, &cfg, "C")
	}

	time.Sleep(1 * time.Hour)
}
