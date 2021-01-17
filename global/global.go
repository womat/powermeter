package global

import (
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/womat/debug"

	"powermeter/pkg/energy"
)

// VERSION holds the version information with the following logic in mind
//  1 ... fixed
//  0 ... year 2020, 1->year 2021, etc.
//  7 ... month of year (7=July)
//  the date format after the + is always the first of the month
//
// VERSION differs from semantic versioning as described in https://semver.org/
// but we keep the correct syntax.
//TODO: increase version number to 1.0.1+2020xxyy
const VERSION = "1.0.5+20210117"
const MODULE = "powermeter"

type DebugConf struct {
	File io.WriteCloser
	Flag int
}

type RegisterConf struct {
	Address uint16
	Format  int
	SF      int
	Mul     float64
	Value   interface{}
}

type Meter struct {
	Type       string
	Connection string
	Measurand  map[string]string
}

type WebserverConf struct {
	Active      bool
	Port        int
	Webservices map[string]bool
}

type CsvConf struct {
	Path             string
	FilenameFormat   string
	Separator        string
	DecimalSeparator string
	Dateformat       string
}

type InfluxConf struct {
	ServerURL string
	User      string
	Password  string
	Location  string
	Database  string
}

type Configuration struct {
	TimerPeriod time.Duration
	Debug       DebugConf
	Meter       map[string]Meter
	Measurand   map[string]map[string]string
	Webserver   WebserverConf
	Csv         CsvConf
	Influx      InfluxConf
}

type Value struct {
	Value     float64
	Delta     float64
	Avg       float64
	LastValue float64
}

type MeteR struct {
	sync.RWMutex `json:"-"`
	Measurand    map[string]*Value
	Handler      energy.Meter `json:"-"`
	LastTime     time.Time
	Time         time.Time
}

type Meters struct {
	sync.RWMutex
	Meter map[string]*MeteR
	Time  time.Time
}

// Config holds the global configuration
var Config Configuration
var AllMeters Meters

func init() {
	Config = Configuration{
		Meter:     map[string]Meter{},
		Measurand: map[string]map[string]string{},
		Webserver: WebserverConf{Webservices: map[string]bool{}},
	}

	AllMeters = Meters{Meter: map[string]*MeteR{}}
}

func (m *MeteR) Reader() (val map[string]float64, err error) {
	m.Lock()
	defer m.Unlock()

	val = map[string]float64{}

	for _, n := range m.Handler.ListMeasurand() {
		val[n], err = m.Handler.GetMeteredValue(n)
		if err != nil {
			debug.ErrorLog.Printf("GetMeteredValue(%q): %v", n, err)
			continue
		}
	}

	return
}

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

func GetField(v interface{}, connectionString, param string) {
	switch param {
	case "baseUrl", "connection":
		fields := strings.Fields(connectionString)
		for _, field := range fields {
			// check if connection string is valid
			//TODO: support dns names!
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
