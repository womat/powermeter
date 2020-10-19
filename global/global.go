package global

import (
	"io"
	"time"
)

// VERSION holds the version information with the following logic in mind
//  1 ... fixed
//  0 ... year 2020, 1->year 2021, etc.
//  7 ... month of year (7=July)
//  the date format after the + is always the first of the month
//
// VERSION differs from semantic versioning as described in https://semver.org/
// but we keep the correct syntax.
const VERSION = "0.0.0+20201008"

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

type Measurand struct {
	Name, Type string
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
	ServerURL   string
	User        string
	Password    string
	Location    string
	Database    string
	Measurement string
}

type Configuration struct {
	TimerPeriod time.Duration
	Debug       DebugConf
	Meter       map[string]Meter
	Measurand   map[string]Measurand
	Webserver   WebserverConf
	Csv         CsvConf
	Influx      InfluxConf
}

// Config holds the global configuration
var Config Configuration

func init() {
	Config = Configuration{
		Meter:     map[string]Meter{},
		Measurand: map[string]Measurand{},
		Webserver: WebserverConf{Webservices: map[string]bool{}},
	}
}
