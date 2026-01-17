// Package greenhttp provides a wrapper around the standard http.Client for making HTTP requests.
// It abstracts HTTP request creation and execution, making it easier to perform HTTP operations
// with custom headers and request bodies throughout the downloader application.
package greenhttp

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates and returns a new HTTPClient instance with a default http.Client.
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{},
	}
}

// DoRequest executes the provided HTTP request using the underlying http.Client.
// It returns the HTTP response or an error if the request fails.
func (c *HTTPClient) DoRequest(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// NewRequest creates a new HTTP request with the specified method, URL, headers, and optional body.
// It returns the constructed http.Request or an error if the request cannot be created.
func (c *HTTPClient) NewRequest(method, url string, headers map[string]string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	if body != nil {
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	}

	return req, nil
}

// Do executes an HTTP request with the specified method, URL, and headers.
// It creates the request, sends it, and returns the response or an error.
func (c *HTTPClient) Do(method, url string, headers map[string]string) (*http.Response, error) {
	req, err := c.NewRequest(method, url, headers, nil)

	if err != nil {
		return nil, err
	}

	resp, err := c.DoRequest(req)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
