package fritzbox

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "http://fritz.box/"
)

var (
	// ErrInvalidStatus is the error returned by request when
	// the command can not be executed
	ErrInvalidStatus = errors.New("fritzbox: wrong status code")
)

// A Client manages communication with the FRITZ!Box
type Client struct {
	// HTTP client used to communicate with the FRITZ!Box
	client *http.Client
	// Base URL for requests. Defaults to the local fritzbox, but
	// can be set to a domain endpoint to use with an external FRITZ!Box.
	// BaseURL should always be specified with a trailing slash.
	BaseURL *url.URL
	// Session used to authenticate client
	session *Session
	// Timeout specifies a time limit for requests made by this
	// Client. The timeout includes connection time, any
	// redirects, and reading the response body. The timer remains
	// running after Get, Head, Post, or Do return and will
	// interrupt reading of the Response.Body.
	TimeOut time.Duration
}

// NewClient returns a new FRITZ!Box client. If a nil httpClient is
// provided, http.DefaultClient will be used. To use an external
// FRITZ!Box with a self-signed certificate, provide an http.Client
// that will be able to perform insecure connections (such as
// InsecureSkipVerify flag).
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	baseURL, _ := url.Parse(defaultBaseURL)

	return &Client{
		client:  httpClient,
		BaseURL: baseURL,
	}
}

// NewRequest creates an API request. A relative URL can be provided
// in urlStr in which case it is resolved relative to the BaseURL of
// the Client. Relative URLs should always be specified without a
// preceding slash. If specified, the value pointed to by data is Query
// encoded and included as the request body in order to perform form requests.
func (c *Client) NewRequest(method, urlStr string, data url.Values) (req *http.Request, err error) {
	var rel *url.URL
	if rel, err = url.Parse(urlStr); err != nil {
		return
	}

	u := c.BaseURL.ResolveReference(rel)

	if c.session != nil {
		values := u.Query()
		values.Set("sid", c.session.Sid)
		u.RawQuery = values.Encode()
	}

	var buf io.Reader
	if data != nil {
		buf = strings.NewReader(data.Encode())
	}

	if req, err = http.NewRequest(method, u.String(), buf); err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return
}

// Do sends a request and returns the response. The response is
// either JSON decoded or XML encoded and stored in the value
// pointed to by v, or returned as an error, if any.
func (c *Client) Do(req *http.Request, v interface{}) (resp *http.Response, err error) {
	if c.session != nil {
		if err = c.session.Refresh(); err != nil {
			return
		}
	}

	if resp, err = c.client.Do(req); err != nil {
		return
	}
	defer resp.Body.Close()

	if c := resp.StatusCode; 200 < c && c > 299 {
		err = ErrInvalidStatus
		return
	}

	contentType := resp.Header.Get("Content-Type")

	if v != nil {
		if strings.Contains(contentType, "text/xml") {
			err = xml.NewDecoder(resp.Body).Decode(v)
		}
		if strings.Contains(contentType, "application/json") {
			err = json.NewDecoder(resp.Body).Decode(v)
		}
		if strings.Contains(contentType, "text/plain") {
			body, e := io.ReadAll(resp.Body)
			if e != nil {
				return resp, e
			}
			switch d := v.(type) {
			case []byte:
				copy(body, d)
			case *float64:
				*d, e = strconv.ParseFloat(strings.TrimSpace(string(body)), 64)
				err = e
			}
		}
	}

	return
}

// Auth sends a auth request and returns an error, if any. Session is stored
// in client in order to perform requests with authentication.
func (c *Client) Auth(username, password string) (err error) {
	if c.session == nil {
		c.session = NewSession(c)
	}

	if err = c.session.Open(); err != nil {
		return
	}

	return c.session.Auth(username, password)
}

// Close closes the current session
func (c *Client) Close() {
	c.session.Close()
}

func (c *Client) String() string {
	return c.session.String()
}
