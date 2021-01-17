package influx

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	influxClient := New()
	influxClient.Open("http://raspberrypi:8086", "admin:", "myhome_test")
	influxClient.AddTag("location", "Wullersdorf")
	influxClient.SetMeasurement("test")
	influxClient.SetTime(time.Now())
	r := []map[string]interface{}{{"f1": 1.2, "f2": "hallo"}, {"f1": 2.2, "f2": "hallo1"}}
	_ = influxClient.Write(r)
	influxClient.Close()
}
