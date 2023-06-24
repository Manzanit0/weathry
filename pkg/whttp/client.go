package whttp

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/exp/slog"
)

type LoggingRoundTripper struct {
	Proxied http.RoundTripper
}

func (lrt LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	q := req.URL.Query()
	q.Set("access_key", "*****")

	slog.Info("sending request",
		"http.method", req.Method,
		"http.url.scheme", req.URL.Scheme,
		"http.url.host", req.URL.Host,
		"http.url.path", req.URL.Path,
		"http.url.query_params", q,
		"http.content_length", req.ContentLength,
		"http.headers", req.Header)

	res, err := lrt.Proxied.RoundTrip(req)
	if err != nil {
		slog.Error(err.Error())
		return res, err
	}

	b := bytes.NewBuffer(make([]byte, 0))
	reader := io.TeeReader(res.Body, b)

	body, _ := ioutil.ReadAll(reader)

	// FIXME: the body will be encoded in log output.
	slog.Info("received response", "http.status_code", res.StatusCode, "http.body", string(body))

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
