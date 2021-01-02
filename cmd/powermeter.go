package main

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"powermeter/global"
	_ "powermeter/pkg/config"
	"powermeter/pkg/csv"
	"powermeter/pkg/debug"
	"powermeter/pkg/fritz"
	"powermeter/pkg/influx"
	"powermeter/pkg/mbclient"
	"powermeter/pkg/mbgw"
	_ "powermeter/pkg/webservice"
)

func main() {
	for meterName, meterConfig := range global.Config.Meter {
		switch t := meterConfig.Type; t {
		case "mbclient":
			c := mbclient.NewClient()
			if err := c.Listen(meterConfig.Connection); err != nil {
				debug.ErrorLog.Printf("error to start modbus client %v: %v\n", meterConfig.Connection, err)
				return
			}

			global.AllMeters.Meter[meterName] = &global.MeteR{Measurand: map[string]*global.Value{}, Handler: c}
		case "mbgateway":
			c := mbgw.NewClient()
			if err := c.Listen(meterConfig.Connection); err != nil {
				debug.ErrorLog.Printf("error to start modbus gateway client %v: %v\n", meterConfig.Connection, err)
				return
			}

			global.AllMeters.Meter[meterName] = &global.MeteR{Measurand: map[string]*global.Value{}, Handler: c}
		case "fritz!powerline":
			c := fritz.NewClient()
			if err := c.Listen(meterConfig.Connection); err != nil {
				debug.ErrorLog.Printf("error to start fritz!powerline client %v: %v\n", meterConfig.Connection, err)
				return
			}
			global.AllMeters.Meter[meterName] = &global.MeteR{Measurand: map[string]*global.Value{}, Handler: c}
		default:
			debug.WarningLog.Printf("client type %q is not supported\n", t)
		}

		global.AllMeters.Meter[meterName].Handler.AddMeasurand(meterConfig.Measurand)
		for _, measurandName := range global.AllMeters.Meter[meterName].Handler.ListMeasurand() {
			global.AllMeters.Meter[meterName].Measurand[measurandName] = &global.Value{}
		}
	}

	ticker := time.NewTicker(global.Config.TimerPeriod)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		runTime := time.Now()

		for meterName, meter := range global.AllMeters.Meter {
			values, err := meter.Reader()
			if err != nil {
				debug.ErrorLog.Printf("error read values from meter %q: %v\n", meterName, err)
			}

			func() {
				meter.Lock()
				defer meter.Unlock()

				meter.LastTime = meter.Time
				meter.Time = time.Now()

				for measurandName, newValue := range values {
					if _, ok := global.Config.Measurand[measurandName]; !ok {
						debug.ErrorLog.Printf("can't find global.Config.Measurand[%q]", measurandName)
						continue
					}

					v := meter.Measurand[measurandName]
					v.LastValue, v.Value = v.Value, newValue
					v.Delta, v.Avg = 0, 0 // math.NaN()

					if v.LastValue > 0 && v.Value != 0 && meter.Time.Sub(meter.LastTime).Hours() < 24*365*10 {
						v.Delta = v.Value - v.LastValue
						v.Avg = v.Delta / meter.Time.Sub(meter.LastTime).Hours()
					}
				}
			}()
		}

		func() {
			global.AllMeters.Lock()
			defer global.AllMeters.Unlock()
			global.AllMeters.LastTime = global.AllMeters.Time
			global.AllMeters.Time = time.Now()
		}()

		debug.InfoLog.Println("runtime to receive data: ", time.Since(runTime))
		runTime = time.Now()

		if err := WriteToCSV(&global.AllMeters, &global.Config); err != nil {
			debug.ErrorLog.Printf("writing to CSV file: %q\n", err)
		}
		if err := WriteToInflux(&global.AllMeters, &global.Config); err != nil {
			debug.ErrorLog.Printf("writing to influx db: %q\n", err)
		}

		debug.InfoLog.Println("runtime to write data: ", time.Since(runTime))
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

func WriteToInflux(m *global.Meters, config *global.Configuration) error {
	influxClient := influx.New()
	influxClient.Open(config.Influx.ServerURL, fmt.Sprintf("%s:%s", config.Influx.User, config.Influx.Password), config.Influx.Database)
	defer influxClient.Close()

	m.RLock()
	defer m.RUnlock()
	influxClient.AddTag("location", config.Influx.Location)
	influxClient.SetTime(m.Time)

	for meterName, meter := range m.Meter {
		records := make([]map[string]interface{}, 1)
		record := map[string]interface{}{}

		for measurandName, measurand := range meter.Measurand {
			if cfgRecords, ok := config.Measurand[measurandName]; ok {
				for n, cfgRecord := range cfgRecords {
					var out string
					getField(&out, cfgRecord, "out")
					if isIn("influx", out) {
						var t string
						getField(&t, cfgRecord, "type")

						switch t {
						case "value":
							record[n] = measurand.Value
						case "delta":
							record[n] = measurand.Delta
						case "avg":
							record[n] = measurand.Avg
						}
					}
				}
			}
		}

		records[0] = record
		if len(records[0]) > 0 {
			influxClient.SetMeasurement(meterName)
			if err := influxClient.Write(records); err != nil {
				return err
			}
		}
	}

	return nil
}

func WriteToCSV(m *global.Meters, config *global.Configuration) error {
	csvFileName := path.Join(config.Csv.Path, csv.FileName(config.Csv.FilenameFormat, time.Now()))
	csvWriter := csv.New()
	csvWriter.ValueSeparator = rune(config.Csv.Separator[0])
	csvWriter.DecimalSeparator = rune(config.Csv.DecimalSeparator[0])
	if err := csvWriter.Open(csvFileName); err != nil {
		return fmt.Errorf("open file %v: %w", csvFileName, err)
	}
	defer csvWriter.Close()

	record := map[string]interface{}{}

	m.RLock()
	defer m.RUnlock()

	record["Date"] = m.Time
	for meterName, meter := range m.Meter {
		for measurandName, measurand := range meter.Measurand {
			if cfgRecords, ok := config.Measurand[measurandName]; ok {
				for n, cfgRecord := range cfgRecords {
					var out string
					getField(&out, cfgRecord, "out")
					if isIn("csv", out) {
						var t string
						headerName := meterName + "-" + n
						getField(&t, cfgRecord, "type")
						switch t {
						case "value":
							record[headerName] = measurand.Value
						case "delta":
							record[headerName] = measurand.Delta
						case "avg":
							record[headerName] = measurand.Avg
						}
					}
				}
			}
		}
	}

	if csvWriter.IsNewFile() {
		if err := csvWriter.WriteOnlyHeader(record); err != nil {
			return fmt.Errorf("write header %q: %w", record, err)
		}
	}

	r := make([]map[string]interface{}, 0, 1)
	r = append(r, record)

	if err := csvWriter.Write(r); err != nil {
		return fmt.Errorf("write csv file: %w", err)
	}

	return nil
}
