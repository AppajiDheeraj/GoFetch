package greenhttp

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

type HTTPClient struct {
	client *http.Client
}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{},
	}
}

func (c *HTTPClient) DoRequest(req *http.Request) (*http.Response, error){
	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *HTTPClient) NewRequest(method, url string, headers map[string]string, body []byte)(*http.Request,error){
	req, err := http.NewRequest(method,url,nil)
	if err != nil {
		return nil, err
	}

	for key, val := range headers {
		req.Header.Set(key,val)
	}

	if(body != nil) {
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	}

	return req, nil
}

func (c* HTTPClient) Do(method, url string, headers map[string]string)(*http.Response, error){
	req, err := c.NewRequest(method,url,headers,nil)

	if err != nil {
		return nil,err
	}

	resp,err := c.DoRequest(req)

	if err != nil {
		return nil, err
	}

	return resp, nil
}