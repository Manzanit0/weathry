package whttp

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type LoggingRoundTripper struct {
	Proxied http.RoundTripper
}

func (lrt LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// TODO: remove api keys from query params
	log.Printf("%s %v\n", req.Method, req.URL)

	res, err := lrt.Proxied.RoundTrip(req)
	if err != nil {
		log.Printf("Error: %v", err)
		return res, err
	}

	b := bytes.NewBuffer(make([]byte, 0))
	reader := io.TeeReader(res.Body, b)

	body, _ := ioutil.ReadAll(reader)
	log.Printf("Received %s response\n%s\n", res.Status, string(body))

	defer res.Body.Close()

	res.Body = ioutil.NopCloser(b)

	return res, nil
}

func NewLoggingClient() *http.Client {
	return &http.Client{
		Transport: LoggingRoundTripper{Proxied: http.DefaultTransport},
		Timeout:   10 * time.Second,
	}
}
