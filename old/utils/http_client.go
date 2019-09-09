package utils

import "net/http"

//method get, post
func HttpMethod(url, method string) error {
	client := &http.Client{}
	request, err := http.NewRequest(method, url, nil)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	_, err = client.Do(request)
	if err != nil {
		return err
	}
	return nil
}
