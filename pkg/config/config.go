package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/womat/debug"
	"github.com/womat/tools"

	"powermeter/global"
)

func init() {
	type yamlStruct struct {
		TimePeriod int
		Debug      struct {
			File string
			Flag string
		}
		Meter map[string]struct {
			Type       string
			Connection string
			Measurand  map[string]string
			Mqtt       global.MqttTopic
		}
		Measurand map[string]map[string]string
		Webserver global.WebserverConf
		Csv       global.CsvConf
		Influx    global.InfluxConf
		Mqtt      global.Mqtt
	}

	var configFile yamlStruct

	flag.Bool("version", false, "print version and exit")
	flag.String("debug.file", "stderr", "log file eg. "+filepath.Join("/opt/womat/log/", global.MODULE+".log"))
	flag.String("debug.flag", "", "enable debug information (standard | trace | debug)")
	flag.String("config", "", "Config File eg. "+filepath.Join("/opt/womat/", global.MODULE+".yaml"))

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
		viper.SetConfigName(global.MODULE)
		viper.AddConfigPath(".")
		viper.AddConfigPath("/opt/womat/")
		viper.AddConfigPath(filepath.Join("/opt/womat/", global.MODULE))
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
	err := viper.Unmarshal(&configFile)
	if err != nil {
		log.Fatalf("unable to decode into struct %v", err)
	}

	getDebugFlag := func(flag string) int {
		switch flag {
		case "trace":
			return debug.Full
		case "debug":
			return debug.Warning | debug.Info | debug.Error | debug.Fatal | debug.Debug
		case "standard":
			return debug.Standard
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
			log.Println(err)
			os.Exit(0)
		}
	}

	for meterName, meter := range configFile.Meter {
		global.Config.Meter[meterName] = global.Meter{
			Type:       meter.Type,
			Connection: meter.Connection,
			Measurand:  meter.Measurand,
			Mqtt:       meter.Mqtt,
		}
	}

	for name, m := range configFile.Measurand {
		global.Config.Measurand[name] = m
	}

	global.Config.TimerPeriod = 5 * time.Second
	if configFile.TimePeriod > 0 {
		global.Config.TimerPeriod = time.Duration(configFile.TimePeriod) * time.Second
	}

	global.Config.Csv = configFile.Csv
	global.Config.Influx = configFile.Influx
	global.Config.Webserver = configFile.Webserver
	global.Config.Mqtt = configFile.Mqtt
}
