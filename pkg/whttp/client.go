package whttp

import (
	"fmt"
	"net/http"
	"time"

	"log/slog"
)

type LoggingRoundTripper struct {
	Proxied http.RoundTripper
}

func (lrt LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	t0 := time.Now()

	res, err := lrt.Proxied.RoundTrip(req)
	if err != nil {
		slog.ErrorContext(req.Context(), err.Error())
		return res, err
	}

	requestDuration := time.Since(t0).Milliseconds()

	// Ofuscate access_key if it exists.
	q := req.URL.Query()
	if q.Has("access_key") {
		q.Set("access_key", "*****")
	}

	// Note: this is useful if we want to print the body.
	// b := bytes.NewBuffer(make([]byte, 0))
	// reader := io.TeeReader(res.Body, b)
	// body, _ := ioutil.ReadAll(reader)
	// defer res.Body.Close()
	// res.Body = ioutil.NopCloser(b)

	msg := fmt.Sprintf("%s %s://%s%s -> %d (%d ms)", req.Method, req.URL.Scheme, req.URL.Host, req.URL.Path, res.StatusCode, requestDuration)
	slog.InfoContext(req.Context(), msg,
		"http.request.duration_ms", requestDuration,
		"http.request.method", req.Method,
		"http.request.url.scheme", req.URL.Scheme,
		"http.request.url.host", req.URL.Host,
		"http.request.url.path", req.URL.Path,
		"http.request.url.query_params", q,
		"http.request.content_length", req.ContentLength,
		"http.request.headers", req.Header,
		"http.response.status_code", res.StatusCode,
		"http.response.headers", res.Header,
		"http.response.content_length", res.ContentLength,
		"http.response.uncompressed", res.Uncompressed,
		"http.response.protocol", res.Proto)

	return res, nil
}

func NewLoggingClient() *http.Client {
	return &http.Client{
		Transport: LoggingRoundTripper{Proxied: http.DefaultTransport},
		Timeout:   10 * time.Second,
	}
}
