package httpclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const defaultMaxResponseBytes int64 = 4 << 20

type Client struct {
	httpClient       *http.Client
	maxResponseBytes int64
}

func New(timeout time.Duration) *Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
	}
	return &Client{httpClient: &http.Client{Transport: transport, Timeout: timeout}, maxResponseBytes: defaultMaxResponseBytes}
}

func (c *Client) Do(ctx context.Context, request *http.Request) ([]byte, int, error) {
	request = request.Clone(ctx)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	reader := io.LimitReader(response.Body, c.maxResponseBytes+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, response.StatusCode, err
	}
	if int64(len(body)) > c.maxResponseBytes {
		return nil, response.StatusCode, fmt.Errorf("response exceeds %d bytes", c.maxResponseBytes)
	}
	return body, response.StatusCode, nil
}
