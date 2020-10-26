package main

import (
	"fmt"
	"math"
	"path"
	"powermeter/global"
	_ "powermeter/pkg/config"
	"powermeter/pkg/csv"
	"powermeter/pkg/debug"
	"powermeter/pkg/energy"
	"powermeter/pkg/fritz"
	"powermeter/pkg/influx"
	"powermeter/pkg/mbclient"
	"powermeter/pkg/mbgw"
	"strconv"
	"strings"
	"time"
)

type value struct {
	value     float64
	delta     float64
	avg       float64
	lastValue float64
}

type meter struct {
	measurand map[string]value
	handler   energy.Meter
}

type meters struct {
	meter    map[string]meter
	lastTime time.Time
	time     time.Time
}

func main() {
	AllMeters := meters{meter: map[string]meter{}}

	for meterName, meterConfig := range global.Config.Meter {
		switch t := meterConfig.Type; t {
		case "mbclient":
			c := mbclient.NewClient()
			if err := c.Listen(meterConfig.Connection); err != nil {
				debug.ErrorLog.Printf("error to start modbus client %v: %v\n", meterConfig.Connection, err)
				return
			}

			AllMeters.meter[meterName] = meter{handler: c, measurand: map[string]value{}}
		case "mbgateway":
			c := mbgw.NewClient()
			if err := c.Listen(meterConfig.Connection); err != nil {
				debug.ErrorLog.Printf("error to start modbus gateway client %v: %v\n", meterConfig.Connection, err)
				return
			}

			AllMeters.meter[meterName] = meter{handler: c, measurand: map[string]value{}}
		case "fritz!powerline":
			c := fritz.NewClient()
			if err := c.Listen(meterConfig.Connection); err != nil {
				debug.ErrorLog.Printf("error to start fritz!powerline client %v: %v\n", meterConfig.Connection, err)
				return
			}
			AllMeters.meter[meterName] = meter{handler: c, measurand: map[string]value{}}
		default:
			debug.WarningLog.Printf("client type %v is not supported\n", t)
		}

		AllMeters.meter[meterName].handler.AddMeasurand(meterConfig.Measurand)
		for _, measurandName := range AllMeters.meter[meterName].handler.ListMeasurand() {
			AllMeters.meter[meterName].measurand[measurandName] = value{}
		}
	}

	ticker := time.NewTicker(global.Config.TimerPeriod)
	defer ticker.Stop()

	for {
		AllMeters.lastTime = AllMeters.time
		AllMeters.time = time.Now()

		for meterName, meter := range AllMeters.meter {
			for _, measurandName := range meter.handler.ListMeasurand() {
				if _, ok := global.Config.Measurand[measurandName]; !ok {
					continue
				}

				tempValue := meter.measurand[measurandName]
				tempValue.lastValue = tempValue.value
				tempValue.value, _ = meter.handler.GetMeteredValue(measurandName)
				tempValue.delta, tempValue.avg = 0, math.NaN()

				if tempValue.lastValue > 0 && tempValue.value != 0 && AllMeters.time.Sub(AllMeters.lastTime).Hours() < 24*365*10 {
					tempValue.delta = tempValue.value - tempValue.lastValue
					tempValue.avg = tempValue.delta / AllMeters.time.Sub(AllMeters.lastTime).Hours()
				}

				meter.measurand[measurandName] = tempValue
			}
			AllMeters.meter[meterName] = meter
		}

		func(m meters) {
			csvFileName := path.Join(global.Config.Csv.Path, csv.FileName(global.Config.Csv.FilenameFormat, time.Now()))
			csvWriter := csv.New()
			csvWriter.ValueSeparator = rune(global.Config.Csv.Separator[0])
			csvWriter.DecimalSeparator = rune(global.Config.Csv.DecimalSeparator[0])
			if err := csvWriter.Open(csvFileName); err != nil {
				debug.ErrorLog.Printf("error open file %v: %v", csvFileName, err)
				return
			}
			defer csvWriter.Close()

			record := map[string]interface{}{}
			record["Date"] = m.time
			for meterName, meter := range m.meter {
				for measurandName, measurand := range meter.measurand {
					if cfgRecords, ok := global.Config.Measurand[measurandName]; ok {
						for n, cfgRecord := range cfgRecords {
							var out string
							getField(&out, cfgRecord, "out")
							if isIn("csv", out) {
								var t string
								headerName := meterName + "-" + n
								getField(&t, cfgRecord, "type")
								switch t {
								case "value":
									record[headerName] = measurand.value
								case "delta":
									record[headerName] = measurand.delta
								case "avg":
									record[headerName] = measurand.avg
								}
							}
						}
					}
				}
			}

			if csvWriter.IsNewFile() {
				if err := csvWriter.WriteOnlyHeader(record); err != nil {
					debug.ErrorLog.Printf("error write header %v: %v", record, err)
					return
				}
			}

			r := make([]map[string]interface{}, 0, 1)
			r = append(r, record)

			if err := csvWriter.Write(r); err != nil {
				debug.ErrorLog.Printf("error write csv file: %v", err)
				return
			}
		}(AllMeters)

		func(m meters) {
			influxClient := influx.New()
			influxClient.Open(global.Config.Influx.ServerURL, fmt.Sprintf("%s:%s", global.Config.Influx.User, global.Config.Influx.Password), global.Config.Influx.Database)
			defer influxClient.Close()

			influxClient.AddTag("location", global.Config.Influx.Location)
			influxClient.SetTime(m.time)

			for meterName, meter := range m.meter {
				records := make([]map[string]interface{}, 1)
				record := map[string]interface{}{}

				for measurandName, measurand := range meter.measurand {
					if cfgRecords, ok := global.Config.Measurand[measurandName]; ok {
						for n, cfgRecord := range cfgRecords {
							var out string
							getField(&out, cfgRecord, "out")
							if isIn("influx", out) {
								var t string
								getField(&t, cfgRecord, "type")

								switch t {
								case "value":
									record[n] = measurand.value
								case "delta":
									record[n] = measurand.delta
								case "avg":
									record[n] = measurand.avg
								}
							}
						}
					}
				}

				records[0] = record

				if len(records[0]) > 0 {
					influxClient.SetMeasurement(meterName)

					if err := influxClient.Write(records); err != nil {
						debug.ErrorLog.Printf("error write to influx db: %v", err)
						return
					}
				}
			}

		}(AllMeters)

		debug.InfoLog.Println("runtime: ", time.Since(AllMeters.time))
		<-ticker.C
	}
}

func getField(v interface{}, connectionString, param string) {

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

func isIn(v, s string) bool {
	fields := strings.FieldsFunc(s, func(c rune) bool { return c == ',' })
	for _, field := range fields {
		if field == v {
			return true
		}
	}
	return false
}
