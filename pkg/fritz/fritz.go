package fritz

import (
	"errors"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"powermeter/pkg/fritzbox"
)

const switchcmdUrlStr = "webservices/homeautoswitch.lua?switchcmd="

var (
	ErrTimeOut = errors.New("fritz: timeout while receiving data")
)

type Client struct {
	baseURL    string
	username   string
	password   string
	ain        string
	timeout    time.Duration
	maxRetries int
	measurand  map[string]mesurandParam
}

type mesurandParam struct {
	command     string
	scaleFactor int
}

// NewClient creates a new Client handler
func NewClient() (c *Client) {
	return &Client{
		measurand: map[string]mesurandParam{},
	}
}

//Listen starts the go function to receive data
func (c *Client) Listen(connectionString string) (err error) {
	//connectionString: http://fritz.box ain:116570149698 username:smarthome password:secret timeout:100 maxretries:0
	getField(&c.baseURL, connectionString, "baseUrl")
	getField(&c.ain, connectionString, "ain")
	getField(&c.username, connectionString, "username")
	getField(&c.password, connectionString, "password")
	getField(&c.timeout, connectionString, "timeout")
	//TODO: auf retry ändern?
	getField(&c.maxRetries, connectionString, "maxretries")
	return
}

func (c *Client) AddMeasurand(measurand map[string]string) {
	for n, m := range measurand {
		p := mesurandParam{}
		getField(&p.command, m, "command")
		getField(&p.scaleFactor, m, "sf")
		c.measurand[n] = p
	}
}

func (c *Client) ListMeasurand() (names []string) {
	for n := range c.measurand {
		names = append(names, n)
	}
	return
}

func (c *Client) GetMeteredValue(measurand string) (e float64, err error) {
	for retryCounter := 0; true; retryCounter++ {
		var v float64

		if err = c.get(switchcmdUrlStr+c.measurand[measurand].command+"&ain="+c.ain, &v); err != nil {
			if retryCounter >= c.maxRetries {
				errorLog.Printf("error to receive client data: %v\n", err)
				return
			}

			warningLog.Printf("error to receive client data: %v\n", err)
			time.Sleep(c.timeout / 2)
			continue
		}

		return v * math.Pow10(c.measurand[measurand].scaleFactor), nil
	}

	return
}

func (c *Client) get(urlStr string, v interface{}) (err error) {
	done := make(chan bool, 1)

	// fills register map with received values or set variable err with error information
	go func() {
		// ensures that data is sent to the channel when the function is terminated
		defer func() {
			select {
			case done <- true:
			default:
			}
			close(done)
		}()
		debugLog.Printf("performing fritz!box http request: %q\n", urlStr)

		fb := fritzbox.NewClient(nil)
		fb.TimeOut = c.timeout

		if fb.BaseURL, err = url.Parse(c.baseURL); err != nil {
			return
		}
		if err = fb.Auth(c.username, c.password); err != nil {
			return
		}
		defer fb.Close()

		var req *http.Request
		if req, err = fb.NewRequest("GET", urlStr, url.Values{}); err != nil {
			return
		}

		_, err = fb.Do(req, v)
	}()

	// wait for  Data
	select {
	case <-done:
	case <-time.After(c.timeout):
		err = ErrTimeOut
	}
	return
}

func (c *Client) Close() (err error) {
	return
}

func getField(v interface{}, connectionString, param string) {

	switch param {
	case "baseUrl":
		fields := strings.Fields(connectionString)
		for _, field := range fields {
			// check if connection string is valid
			if regexp.MustCompile(`^https?://.*$`).MatchString(field) {
				switch x := v.(type) {
				case *string:
					*x = field
				}
				return
			}
		}
	default:
		var value string

		fields := strings.Fields(connectionString)
		for _, field := range fields {
			parts := strings.Split(field, ":")
			if parts[0] != param {
				continue
			}

			if len(parts) == 2 {
				value = parts[1]
			}

			switch x := v.(type) {
			case *string:
				*x = value
			case *int:
				*x, _ = strconv.Atoi(value)
			case *time.Duration:
				*x = time.Second
				if i, err := strconv.Atoi(value); err == nil {
					*x = time.Duration(i) * time.Millisecond

				}
			}
			return
		}
	}
}
