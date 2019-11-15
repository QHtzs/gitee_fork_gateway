package main

import (
	"net"
	"net/http"
	"strings"
	"time"
)

/*
@author: TTG
@brief: 此类函数在关闭连接后触发，执行之后goroutine退出，
故不能阻塞过长时间以免空占资源
非延时操作，避免阻塞时间过长

*/

// http客户端
type NoForeverBlockClient struct {
	Client *http.Client
}

// 设置客户端超时时间
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

//创建访问时会超时的客户端
var globalClient NoForeverBlockClient = CreateClient()

//http get method
func HttpGet(url string) {
	rsp, err := globalClient.Client.Get(url)
	if err == nil {
		defer rsp.Body.Close()
	}
}

//http post method [content-type:]
func HttpPost(url, data string) {
	rsp, err := globalClient.Client.Post(url, "text/html", strings.NewReader(data))
	if err == nil {
		defer rsp.Body.Close()
	}
}

//http post method [content-type:json]
func HttpPostJson(url, json string) {
	rsp, err := globalClient.Client.Post(url, "text/json", strings.NewReader(json))
	if err == nil {
		defer rsp.Body.Close()
	}
}
