package s0counter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/womat/debug"
	"github.com/womat/tools"
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
	_ = tools.GetField(&c.connectionString, connectionString, "connection")
	_ = tools.GetField(&c.timeout, connectionString, "timeout")
	_ = tools.GetField(&c.cacheTime, connectionString, "cachetime")
	_ = tools.GetField(&c.maxRetries, connectionString, "maxretries")
	return
}

func (c *Client) AddMeasurand(measurand map[string]string) {
	for n, m := range measurand {
		p := measurandParam{}
		_ = tools.GetField(&p.key, m, "key")
		_ = tools.GetField(&p.value, m, "value")
		_ = tools.GetField(&p.scaleFactor, m, "sf")
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
		debug.TraceLog.Printf("key %q is not cached\n", m.key)

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
	} else {
		debug.TraceLog.Printf("key %q is cached\n", m.key)
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

		bodyBytes, _ := io.ReadAll(resp.Body)
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

	for a, b := range val {
		b.Timestamp = time.Now()
		val[a] = b
	}

	debug.TraceLog.Printf("api response: %+v\n", val)
	return
}

func (c *Client) Close() (err error) {
	return
}
