package mbclient

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/goburrow/modbus"
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

func NewClient() (c *Client) {
	return &Client{
		measurand: map[string]measurandParam{},
	}
}

// Listen starts the go function to receive data
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
		err = fmt.Errorf("unknow measurand: %v\n", measurand)
		return
	}

	for retryCounter := 0; true; retryCounter++ {
		var v float64
		var data []byte

		q := quantity(m.format)
		if data, err = c.get(m.address, q); err != nil {
			if retryCounter >= c.maxRetries {
				errorLog.Printf("error to receive client data: %v\n", err)
				return
			}

			warningLog.Printf("error to receive client data: %v\n", err)
			time.Sleep(c.timeout / 2)
			continue
		}

		switch d := data[0 : q*2]; m.format {
		case _sint16:
			v = float64(int16(binary.BigEndian.Uint16(d)))
		case _sint32:
			v = float64(int32(binary.BigEndian.Uint32(d)))
		case _sint64:
			v = float64(int64(binary.BigEndian.Uint64(d)))
		case _uint16:
			v = float64(binary.BigEndian.Uint16(d))
		case _uint32:
			v = float64(binary.BigEndian.Uint32(d))
		case _uint64:
			v = float64(binary.BigEndian.Uint64(d))
		case _float32:
			v = float64(math.Float32frombits(binary.BigEndian.Uint32(d)))
		}
		return v * math.Pow10(m.scaleFactor), nil
	}

	return
}

func quantity(format int) int {
	switch format {
	case _sint16, _uint16:
		return 1
	case _sint32, _uint32, _float32:
		return 2
	case _sint64, _uint64:
		return 4
	}
	return 0
}
func (c *Client) get(address uint16, quantity int) (data []byte, err error) {
	done := make(chan bool, 1)

	//  fills register map with received values or set variable err with error information
	go func() {
		// ensures that data is sent to the channel when the function is terminated
		defer func() {
			select {
			case done <- true:
			default:
			}
			close(done)
		}()

		clientHandler := modbus.NewTCPClientHandler(c.connectionString)
		clientHandler.SlaveId = c.deviceId

		if err = clientHandler.Connect(); err != nil {
			return
		}
		defer clientHandler.Close()

		client := modbus.NewClient(clientHandler)
		data, err = client.ReadHoldingRegisters(address, uint16(quantity))
	}()

	// wait for Modbus Data
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
