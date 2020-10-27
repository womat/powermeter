package mbgw

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
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

// ClientData stores receive data form modbus gateway
type ClientData struct {
	Timestamp time.Time
	Runtime   time.Duration
	Register  map[uint16]uint16
}

// Client structure contains all Properties of a connection
type Client struct {
	connectionString string
	timeout          time.Duration
	deviceId         uint8
	maxRetries       int
	measurand        map[string]measurandParam
}

type measurandParam struct {
	address     uint16
	format      int
	scaleFactor int
}

// NewClient creates a new Client handler
func NewClient() (c *Client) {
	return &Client{
		timeout:   time.Second,
		deviceId:  1,
		measurand: map[string]measurandParam{},
	}
}

//Listen starts the go function to receive data
func (c *Client) Listen(connectionString string) (err error) {
	getField(&c.connectionString, connectionString, "connection")
	getField(&c.deviceId, connectionString, "deviceid")
	getField(&c.timeout, connectionString, "timeout")
	//TODO: auf retry ändern?
	getField(&c.maxRetries, connectionString, "maxretries")
	return
}

func (c *Client) AddMeasurand(measurand map[string]string) {
	for n, m := range measurand {
		p := measurandParam{
			format: _uint16,
		}
		getField(&p.address, m, "address")
		getField(&p.scaleFactor, m, "sf")
		getField(&p.format, m, "format")
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
	var m measurandParam
	var ok bool

	if m, ok = c.measurand[measurand]; !ok {
		err = fmt.Errorf("unknow measurand: %v", measurand)
		return
	}

	for retryCounter := 0; true; retryCounter++ {
		var v float64
		var register map[uint16]uint16

		connectionString := fmt.Sprintf("%v/readholdingregisters?Address=%v&Quantity=%v", c.connectionString, m.address, quantity(m.format))
		if register, err = c.get(connectionString); err != nil {
			if retryCounter >= c.maxRetries {
				errorLog.Printf("error to receive client data: %v\n", err)
				return
			}

			warningLog.Printf("error to receive client data: %v\n", err)
			time.Sleep(c.timeout / 2)
			continue
		}

		switch m.format {
		case _sint16:
			v = int16ToFloat64(register)
		case _sint32:
			v = int32ToFloat64(register)
		case _sint64:
			v = int64ToFloat64(register)
		case _uint16:
			v = uint16ToFloat64(register)
		case _uint32:
			v = uint32ToFloat64(register)
		case _uint64:
			v = uint64ToFloat64(register)
		case _float32:
			v = bitsToFloat64(register)
		}
		return v * math.Pow10(m.scaleFactor), nil
	}

	return
}

func uint16ToFloat64(r map[uint16]uint16) float64 {
	return float64(binary.BigEndian.Uint16(getBytes(r)))
}
func uint32ToFloat64(r map[uint16]uint16) float64 {
	return float64(binary.BigEndian.Uint32(getBytes(r)))
}
func uint64ToFloat64(r map[uint16]uint16) float64 {
	return float64(binary.BigEndian.Uint64(getBytes(r)))
}

func int16ToFloat64(r map[uint16]uint16) float64 {
	return float64(int16(uint16ToFloat64(r)))
}
func int32ToFloat64(r map[uint16]uint16) float64 {
	return float64(int32(uint32ToFloat64(r)))
}
func int64ToFloat64(r map[uint16]uint16) float64 {
	return float64(int64(uint64ToFloat64(r)))
}

func bitsToFloat64(r map[uint16]uint16) float64 {
	return float64(math.Float32frombits(binary.BigEndian.Uint32(getBytes(r))))
}

func getBytes(r map[uint16]uint16) []byte {
	b := make([]byte, len(r)*2)

	keys := make([]int, 0, len(r))
	for k := range r {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)

	for i, k := range keys {
		binary.BigEndian.PutUint16(b[i*2:i*2+2], r[uint16(k)])
	}

	return b
}

func (c *Client) get(connectionString string) (register map[uint16]uint16, err error) {
	register = make(map[uint16]uint16)
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

		debugLog.Printf("performing http get: %v\n", connectionString)

		var resp *http.Response
		if resp, err = http.Get(connectionString); err != nil {
			return
		}

		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()

		// Convert response body to result struct
		type body struct {
			Time       time.Time
			Duration   int
			Connection string
			Data       struct {
				Address, Quantity uint16
				Data              string
			}
		}

		var bodyStruct body
		if err = json.Unmarshal(bodyBytes, &bodyStruct); err != nil {
			return
		}
		traceLog.Printf("api response: %+v\n", bodyStruct)

		for i := 0; i < int(bodyStruct.Data.Quantity); i++ {
			var value uint64
			if value, err = strconv.ParseUint(bodyStruct.Data.Data[i*4:i*4+4], 16, 16); err != nil {
				return
			}
			register[bodyStruct.Data.Address+uint16(i)] = uint16(value)
		}
	}()

	// wait for API Data
	select {
	case <-done:
	case <-time.After(c.timeout):
		err = errors.New("timeout during receive data")
	}
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
			//TODO: auch dns namen sollen unterstützt werden!
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

func quantity(format int) int {
	switch format {
	case _sint16, _uint16:
		return 1
	case _sint32, _uint32:
		return 2
	case _sint64, _uint64:
		return 4
	}
	return 0
}
