package mbclient

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/goburrow/modbus"

	"powermeter/pkg/tools"
)

// Client structure contains all Properties of a connection
type Client struct {
	connectionString string
	timeout          time.Duration
	deviceId         uint8
	maxRetries       int
}

func NewClient() (c *Client) {
	return &Client{}
}

// Listen starts the go function to receive data
func (c *Client) Listen(connectionString string) (err error) {
	c.connectionString, c.deviceId, c.timeout, c.maxRetries = tools.GetConnectionDeviceIdTimeOut(connectionString)
	return
}

func (c *Client) GetEnergyCounter() (e float64, err error) {
	var retryCounter int
	var data []byte
	for retryCounter = 0; retryCounter <= c.maxRetries; retryCounter++ {

		data, err = c.get(41000-1, 4)
		if err == nil {
			break
		}

		time.Sleep(10 * time.Millisecond)
		errorlog.Printf("error to receive client data: %v\n", err)
	}

	e = float64(binary.BigEndian.Uint64(data[0:8]))
	return
}

func (c *Client) get(address, quantity uint16) (data []byte, err error) {
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
		// TODO registers should be a parameter in the config file
		data, err = client.ReadHoldingRegisters(address, quantity)
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
