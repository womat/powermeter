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

const (
	Polling = iota
	Request
)

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

type ClientConf struct {
	Type       string
	Connection string
	Register   map[string]RegisterConf
}

type WebserverConf struct {
	Active      bool
	Port        int
	Webservices map[string]bool
}

type CsvConf struct {
	Path             string
	Filenameformat   string
	Seperator        string
	Decimalseperator string
	Dateformat       string
}

type InfluxConf struct {
	Port     int
	User     string
	Password string
	Location string
}

type Configuration struct {
	Timerperiod time.Duration
	Debug       DebugConf
	Clients     map[string]ClientConf
	Webserver   WebserverConf
	Csv         CsvConf
	Influx      InfluxConf
}

// Config holds the global configuration
var Config Configuration

func init() {
	Config = Configuration{
		Clients:   map[string]ClientConf{},
		Webserver: WebserverConf{Webservices: map[string]bool{}},
	}
}
