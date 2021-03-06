package internal

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
)

type Client struct {
	http               *http.Client
	endpoint           *url.URL
	username, password string
}

func NewClient(c *http.Client, endpoint string) (*Client, error) {
	if c == nil {
		c = http.DefaultClient
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	return &Client{http: c, endpoint: u}, nil
}

func (c *Client) SetBasicAuth(username, password string) {
	c.username = username
	c.password = password
}

func (c *Client) ResolveHref(p string) *url.URL {
	return &url.URL{
		Scheme: c.endpoint.Scheme,
		User:   c.endpoint.User,
		Host:   c.endpoint.Host,
		Path:   path.Join(c.endpoint.Path, p),
	}
}

func (c *Client) NewRequest(method string, path string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, c.ResolveHref(path).String(), body)
}

func (c *Client) NewXMLRequest(method string, path string, v interface{}) (*http.Request, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	if err := xml.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}

	req, err := c.NewRequest(method, path, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "text/xml; charset=\"utf-8\"")

	return req, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		// TODO: if body is plaintext, read it and populate the error message
		resp.Body.Close()
		return nil, &HTTPError{Code: resp.StatusCode}
	}
	return resp, nil
}

func (c *Client) DoMultiStatus(req *http.Request) (*Multistatus, error) {
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus {
		return nil, fmt.Errorf("HTTP multi-status request failed: %v", resp.Status)
	}

	// TODO: the response can be quite large, support streaming Response elements
	var ms Multistatus
	if err := xml.NewDecoder(resp.Body).Decode(&ms); err != nil {
		return nil, err
	}

	return &ms, nil
}

func (c *Client) Propfind(path string, depth Depth, propfind *Propfind) (*Multistatus, error) {
	req, err := c.NewXMLRequest("PROPFIND", path, propfind)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Depth", depth.String())

	return c.DoMultiStatus(req)
}

// PropfindFlat performs a PROPFIND request with a zero depth.
func (c *Client) PropfindFlat(path string, propfind *Propfind) (*Response, error) {
	ms, err := c.Propfind(path, DepthZero, propfind)
	if err != nil {
		return nil, err
	}

	return ms.Get(path)
}
