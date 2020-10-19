package influx

import (
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"time"
)

// Writer provides API to communicate with InfluxDBServer
type Writer struct {
	client      influxdb2.Client
	writeAPI    api.WriteAPI
	measurement string
	tags        map[string]string
	timestamp   time.Time
}

// New create an API to communicate with InfluxDBServer
func New() *Writer {
	return &Writer{
		tags: map[string]string{},
	}
}

// SetMeasurement set Measurement for a Point.
func (w *Writer) SetMeasurement(m string) {
	w.measurement = m
}

// SetTime set timestamp for a Point.
func (w *Writer) SetTime(timestamp time.Time) {
	w.timestamp = timestamp
}

// AddTag adds a tag to a point.
func (w *Writer) AddTag(k, v string) {
	w.tags[k] = v
}

// Open opens a writer to an influx DB
// serverURL is the InfluxDB server base URL, e.g. http://localhost:8086,
// authToken is an authentication token. It can be empty in case of connecting to newly installed InfluxDB server, which has not been set up yet.
// Use the form username:password for an authentication token.
// Example: my-user:my-password.
// Use an empty string ("") if the server doesn't require authentication.
func (w *Writer) Open(serverURL, authToken, bucket string) {
	w.client = influxdb2.NewClient(serverURL, authToken)
	w.writeAPI = w.client.WriteAPI("", bucket)
}

// Write writes a data point to influx DB
func (w *Writer) Write(records []map[string]interface{}) (err error) {
	for _, fields := range records {
		// create point using full params constructor
		p := influxdb2.NewPoint(w.measurement,
			w.tags,
			fields,
			w.timestamp)
		// write point immediately
		w.writeAPI.WritePoint(p)
	}
	return
}

// Close force all unwritten data to be sent and close
func (w *Writer) Close() {
	// Force all unwritten data to be sent
	w.writeAPI.Flush()
	w.client.Close()
}
