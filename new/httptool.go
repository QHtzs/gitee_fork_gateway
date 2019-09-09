package main

import (
	"net/http"
	"strings"
)

func HttpGet(url string) (*http.Response, error) {
	return http.Get(url)
}

func HttpPost(url, data string) (*http.Response, error) {
	return http.Post(url, "text/html", strings.NewReader(data))
}

func HttpPostJson(url, json string) (*http.Response, error) {
	return http.Post(url, "text/json", strings.NewReader(json))
}
