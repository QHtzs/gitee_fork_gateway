package main

import (
	"net"
	"net/http"
	"strings"
	"time"
)

/*
此类函数在关闭连接后触发，执行之后goroutine退出，
故不能阻塞过长时间以免空占资源
非延时操作，避免阻塞时间过长
*/
type NoForeverBlockClient struct {
	Client *http.Client
}

func CreateClient() NoForeverBlockClient {
	client := NoForeverBlockClient{}
	client.Client = &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, time.Second*2)
				if err != nil {
					return nil, err
				}
				conn.SetDeadline(time.Now().Add(time.Second * 2))
				return conn, nil
			},
			ResponseHeaderTimeout: time.Second * 2,
		},
	}
	return client
}

var globalClient NoForeverBlockClient = CreateClient()

func HttpGet(url string) {
	rsp, err := globalClient.Client.Get(url)
	if err == nil {
		defer rsp.Body.Close()
	}
}

func HttpPost(url, data string) {
	rsp, err := globalClient.Client.Post(url, "text/html", strings.NewReader(data))
	if err == nil {
		defer rsp.Body.Close()
	}
}

func HttpPostJson(url, json string) {
	rsp, err := globalClient.Client.Post(url, "text/json", strings.NewReader(json))
	if err == nil {
		defer rsp.Body.Close()
	}
}
