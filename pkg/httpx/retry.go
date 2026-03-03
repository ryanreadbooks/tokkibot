package httpx

import (
	"io"
	"math"
	"net/http"
	"time"
)

type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   5 * time.Second,
	}
}

type RetryRoundTripper struct {
	transport http.RoundTripper
	cfg       RetryConfig
}

func NewRetryRoundTripper(transport http.RoundTripper, cfg RetryConfig) *RetryRoundTripper {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &RetryRoundTripper{
		transport: transport,
		cfg:       cfg,
	}
}

func (rt *RetryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		resp    *http.Response
		err     error
		bodyBuf []byte
	)

	if req.Body != nil && req.GetBody == nil {
		bodyBuf, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
	}

	for attempt := 0; attempt <= rt.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := rt.calcBackoff(attempt)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay):
			}
		}

		if err = rt.resetRequestBody(req, bodyBuf); err != nil {
			return nil, err
		}

		resp, err = rt.transport.RoundTrip(req)
		if err == nil && !rt.shouldRetry(resp.StatusCode) {
			return resp, nil
		}

		if resp != nil && resp.Body != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}

		if err != nil && !isRetryableError(err) {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (rt *RetryRoundTripper) resetRequestBody(req *http.Request, bodyBuf []byte) error {
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return err
		}
		req.Body = body
	} else if bodyBuf != nil {
		req.Body = io.NopCloser(&bytesReader{buf: bodyBuf})
	}
	return nil
}

func (rt *RetryRoundTripper) calcBackoff(attempt int) time.Duration {
	delay := time.Duration(float64(rt.cfg.BaseDelay) * math.Pow(2, float64(attempt-1)))
	if delay > rt.cfg.MaxDelay {
		delay = rt.cfg.MaxDelay
	}
	return delay
}

func (rt *RetryRoundTripper) shouldRetry(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

func isRetryableError(err error) bool {
	return err != nil
}

type bytesReader struct {
	buf []byte
	off int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.off >= len(r.buf) {
		return 0, io.EOF
	}
	n = copy(p, r.buf[r.off:])
	r.off += n
	return n, nil
}

func NewRetryClient(cfg RetryConfig) *http.Client {
	return &http.Client{
		Transport: NewRetryRoundTripper(http.DefaultTransport, cfg),
	}
}
