package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// Returns an *http.Client which will receive a response containing
// body, headers, and status code specified. Must close received server.
// Proxies all requests made using returned client to a httptest.Server
// which uses an http.Handler to handle requests.
// See: http://keighl.com/post/mocking-http-responses-in-golang/
func MockClient(code int, body []byte, headers map[string]string) (*http.Client, *httptest.Server) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(code)
		w.Write(body)
	}))
	transport := &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}
	return &http.Client{Transport: transport}, server
}

// Same as MockClient but the server uses the defined http.Handler
func MockClientHandler(fn func(http.ResponseWriter, *http.Request)) (*http.Client, *httptest.Server) {
	server := httptest.NewServer(http.HandlerFunc(fn))
	transport := &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}
	return &http.Client{Transport: transport}, server
}

// Calls t.Error with err, and writes status code to response
func ErrorWithCode(t *testing.T, w http.ResponseWriter, err string, code int) {
	w.WriteHeader(code)
	t.Error(err)
}
