package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"powermeter/global"
	"powermeter/pkg/tools"
)

const (
	_nil = iota
	_sint16
	_sint32
	_sint64
	_uint16
	_uint32
	_uint64
)

func init() {
	type yamlStruct struct {
		timeperiod int
		Debug      struct {
			File string
			Flag string
		}
		Clients map[string]struct {
			Type       string
			Connection string
			Register   map[string]string
		}
		Webserver global.WebserverConf
		Csv       global.CsvConf
		Influx    global.InfluxConf
	}

	var configFile yamlStruct

	flag.Bool("version", false, "print version and exit")
	flag.String("debug.file", "stderr", "log file eg. /tmp/emu.log")
	flag.String("debug.flag", "", "enable debug information (standard | trace | debug)")
	flag.String("config", "", "Config File eg. /opt/womat/config.yaml")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	_ = viper.BindPFlags(pflag.CommandLine)

	if viper.GetBool("version") {
		fmt.Printf("Version: %v\n", global.VERSION)
		os.Exit(0)
	}

	if f := viper.GetString("config"); f != "" {
		viper.SetConfigFile(f)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/opt/womat/")
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
	err := viper.Unmarshal(&configFile)
	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}

	getDebugFlag := func(flag string) int {
		switch flag {
		case "trace":
			return Full
		case "debug":
			return Warning | Info | Error | Fatal | Debug
		case "standard":
			return Standard
		}
		return 0
	}

	global.Config.Debug.Flag = getDebugFlag(configFile.Debug.Flag)
	switch file := configFile.Debug.File; file {
	case "stderr":
		global.Config.Debug.File = os.Stderr
	case "stdout":
		global.Config.Debug.File = os.Stdout
	default:
		if !tools.FileExists(file) {
			_ = tools.CreateFile(file)
		}
		if global.Config.Debug.File, err = os.Open(file); err != nil {
			fatallog.Println(err)
			os.Exit(0)
		}
	}
	for clientName, client := range configFile.Clients {
		global.Config.Clients[clientName] = global.ClientConf{
			Type:       client.Type,
			Connection: client.Connection,
			Register:   map[string]global.RegisterConf{},
		}

		for registerName, register := range client.Register {
			global.Config.Clients[clientName].Register[registerName] = getRegisterConfig(register)
		}

		global.Config.Timerperiod = 5 * time.Second
		if configFile.timeperiod > 0 {
			global.Config.Timerperiod = time.Duration(configFile.timeperiod) * time.Second

		}
		global.Config.Csv = configFile.Csv
		global.Config.Influx = configFile.Influx
		global.Config.Webserver = configFile.Webserver

	}

	return
}

func getRegisterConfig(config string) (reg global.RegisterConf) {
	reg.Format = _uint16
	reg.Mul = 1

	m := make(map[string]string)
	fields := strings.Fields(config)

	// split fields into a map, eg Value:99 >> m[Value]=99
	for _, field := range fields {
		parts := strings.Split(field, ":")
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
			continue
		}
		m[parts[0]] = ""
	}

	for p, v := range m {
		switch p {
		case "sint16":
			reg.Format = _sint16
		case "sint32":
			reg.Format = _sint32
		case "sint64":
			reg.Format = _sint64
		case "uint16":
			reg.Format = _uint16
		case "uint32":
			reg.Format = _uint32
		case "uint64":
			reg.Format = _uint64
		case "Addr":
			i, _ := strconv.Atoi(v)
			reg.Address = uint16(i)
		case "SF":
			reg.SF, _ = strconv.Atoi(v)
		case "Mul":
			reg.Mul, _ = strconv.ParseFloat(v, 64)
		case "Value":
			x, _ := strconv.ParseInt(v, 10, 64)
			// since value is of type interface {}, the correct type must be determined
			for format := range m {
				switch format {
				case "sint16":
					reg.Value = int16(x)
				case "sint32":
					reg.Value = int32(x)
				case "sint64":
					reg.Value = x
				case "uint16":
					reg.Value = uint16(x)
				case "uint32":
					reg.Value = uint32(x)
				case "uint64":
					reg.Value = uint64(x)
				}
			}
			if reg.Value == nil {
				//if no type was defined, UINT16 is used (corresponds to modbus register)
				reg.Value = uint16(x)
			}
		}
	}

	return reg
}
