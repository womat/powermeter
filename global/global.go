package global

import (
	"io"
	"powermeter/pkg/debug"
	"sync"
	"time"

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
const VERSION = "1.0.1+20201228"
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
	//TODO: LastTime und Time wird hier nicht benötigt
	LastTime time.Time
	Time     time.Time
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
