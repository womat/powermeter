package s0counter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/womat/debug"
)

const (
	_nil = iota
	_sint16
	_sint32
	_sint64
	_uint16
	_uint32
	_uint64
	_float32
)

// ClientData stores receive data form s0counter web request
type ClientData struct {
	Timestamp        time.Time `json:"TimeStamp"`
	MeterReading     float64   `json:"MeterReading"`
	UnitMeterReading string    `json:"UnitMeterReading"`
	Flow             float64   `json:"Flow"`
	UnitFlow         string    `json:"UnitFlow"`
}

// Client structure contains all Properties of a connection
type Client struct {
	connectionString string
	timeout          time.Duration
	cacheTime        time.Duration
	maxRetries       int
	measurand        map[string]measurandParam
	cache            map[string]ClientData
}

type measurandParam struct {
	key, value  string
	scaleFactor int
}

// NewClient creates a new Client handler
func NewClient() (c *Client) {
	return &Client{
		timeout:   time.Second,
		cacheTime: time.Second,
		measurand: map[string]measurandParam{},
		cache:     map[string]ClientData{},
	}
}

func (c *Client) String() string {
	return "s0counter"
}

// Listen starts the go function to receive data
func (c *Client) Listen(connectionString string) (err error) {
	getField(&c.connectionString, connectionString, "connection")
	getField(&c.timeout, connectionString, "timeout")
	getField(&c.cacheTime, connectionString, "cachetime")
	//TODO: change to auf retry?
	getField(&c.maxRetries, connectionString, "maxretries")
	return
}

func (c *Client) AddMeasurand(measurand map[string]string) {
	for n, m := range measurand {
		p := measurandParam{}
		getField(&p.key, m, "key")
		getField(&p.value, m, "value")
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
	var data ClientData
	var m measurandParam
	var ok bool
	var v float64

	if m, ok = c.measurand[measurand]; !ok {
		err = fmt.Errorf("unknow measurand: %v", measurand)
		return
	}

	if _, ok := c.cache[m.key]; !ok || time.Now().After(c.cache[m.key].Timestamp.Add(c.cacheTime)) {
		debug.DebugLog.Printf("key %q is not cached\n", m.key)

		for retryCounter := 0; true; retryCounter++ {
			if c.cache, err = c.get(c.connectionString); err != nil {
				if retryCounter >= c.maxRetries {
					debug.ErrorLog.Printf("error to receive client data: %v\n", err)
					return
				}

				debug.WarningLog.Printf("error to receive client data: %v\n", err)
				time.Sleep(c.timeout / 2)
				continue
			}
			break
		}
	}

	if data, ok = c.cache[m.key]; !ok {
		err = fmt.Errorf("unknow measurand: %v", measurand)
		return
	}

	switch m.value {
	case "MeterReading":
		v = data.MeterReading
	case "Flow":
		v = data.Flow
	default:
		err = fmt.Errorf("unknow measurand value: %v", measurand)
		return
	}
	return v * math.Pow10(m.scaleFactor), nil
}

func (c *Client) get(connectionString string) (val map[string]ClientData, err error) {
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

		debug.DebugLog.Printf("performing http get: %q\n", connectionString)

		var resp *http.Response
		if resp, err = http.Get(connectionString); err != nil {
			return
		}

		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if err = json.Unmarshal(bodyBytes, &val); err != nil {
			return
		}
		debug.TraceLog.Printf("api response: %+v\n", val)
	}()

	// wait for API Data
	select {
	case <-done:
	case <-time.After(c.timeout):
		err = errors.New("timeout during receive data")
	}

	debug.TraceLog.Printf("api response: %+v\n", val)
	return
}

func (c *Client) Close() (err error) {
	return
}

func getField(v interface{}, connectionString, param string) {
	switch param {
	case "baseUrl", "connection":
		fields := strings.Fields(connectionString)
		for _, field := range fields {
			// check if connection string is valid
			//TODO: support dns names
			if regexp.MustCompile(`^https?://.*$`).MatchString(field) || regexp.MustCompile(`^[\d]{1,3}\.[\d]{1,3}\.[\d]{1,3}\.[\d]{1,3}:[\d]{1,5}$`).MatchString(field) {
				switch x := v.(type) {
				case *string:
					*x = field
				}
				return
			}
		}
	case "format":
		fields := strings.Fields(connectionString)
		var i int
		for _, field := range fields {
			switch field {
			case "sint16":
				i = _sint16
			case "sint32":
				i = _sint32
			case "sint64":
				i = _sint64
			case "uint16":
				i = _uint16
			case "uint32":
				i = _uint32
			case "uint64":
				i = _uint64
			case "float32":
				i = _float32
			default:
				continue
			}

			switch x := v.(type) {
			case *int:
				*x = i
			}
			return
		}

	default:
		fields := strings.Fields(connectionString)
		for _, field := range fields {
			parts := strings.Split(field, ":")
			if parts[0] != param || len(parts) != 2 {
				continue
			}

			value := parts[1]

			switch x := v.(type) {
			case *string:
				*x = value
			case *int:
				*x, _ = strconv.Atoi(value)
			case *uint16:
				i, _ := strconv.Atoi(value)
				*x = uint16(i)
			case *uint8:
				i, _ := strconv.Atoi(value)
				*x = uint8(i)
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
