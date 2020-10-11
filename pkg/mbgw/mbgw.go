package mbgw

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"powermeter/pkg/tools"
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
}

// NewClient creates a new Client handler
func NewClient() (c *Client) {
	return &Client{}
}

//Listen starts the go function to receive data
func (c *Client) Listen(connectionString string) (err error) {
	c.connectionString, c.deviceId, c.timeout, c.maxRetries = tools.GetConnectionDeviceIdTimeOut(connectionString)
	return
}

func (c *Client) GetEnergyCounter() (e float64, err error) {
	var retryCounter int
	var register map[uint16]uint16
	for retryCounter = 0; retryCounter <= c.maxRetries; retryCounter++ {

		connectionString := fmt.Sprintf("%v/readholdingregisters?Address=%v&Quantity=%v", c.connectionString, 4124, 2)
		register, err = c.get(connectionString)

		if err == nil {
			break
		}

		time.Sleep(10 * time.Millisecond)
		errorlog.Printf("error to receive client data: %v\n", err)
	}

	e = float64(uint32(register[4124])<<16 | uint32(register[4125]))
	return
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

		debuglog.Printf("performing http get: %v\n", connectionString)

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
		tracelog.Printf("api response: %+v\n", bodyStruct)

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
