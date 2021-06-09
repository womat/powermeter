package main

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/womat/debug"
	"github.com/womat/tools"

	"powermeter/global"
	_ "powermeter/pkg/config"
	"powermeter/pkg/csv"
	"powermeter/pkg/fritz"
	"powermeter/pkg/influx"
	"powermeter/pkg/mbclient"
	"powermeter/pkg/mbgw"
	"powermeter/pkg/s0counter"
	_ "powermeter/pkg/webservice"
)

func main() {
	debug.SetDebug(global.Config.Debug.File, global.Config.Debug.Flag)

	if err := initMeter(); err != nil {
		debug.ErrorLog.Println(err)
		return
	}

	ticker := time.NewTicker(global.Config.TimerPeriod)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		loop()
	}
}

func initMeter() (err error) {
	for meterName, meterConfig := range global.Config.Meter {
		switch t := meterConfig.Type; t {
		case "mbclient":
			c := mbclient.NewClient()
			global.AllMeters.Meter[meterName] = &global.MeteR{Measurand: map[string]*global.Value{}, Handler: c}
		case "mbgateway":
			c := mbgw.NewClient()
			global.AllMeters.Meter[meterName] = &global.MeteR{Measurand: map[string]*global.Value{}, Handler: c}
		case "fritz!powerline":
			c := fritz.NewClient()
			global.AllMeters.Meter[meterName] = &global.MeteR{Measurand: map[string]*global.Value{}, Handler: c}
		case "s0counter":
			c := s0counter.NewClient()
			global.AllMeters.Meter[meterName] = &global.MeteR{Measurand: map[string]*global.Value{}, Handler: c}
		default:
			debug.WarningLog.Printf("client type %q is not supported\n", t)
		}

		if err := global.AllMeters.Meter[meterName].Handler.Listen(meterConfig.Connection); err != nil {
			return fmt.Errorf("error to start %v client %v: %w", global.AllMeters.Meter[meterName].Handler, meterConfig.Connection, err)
		}

		global.AllMeters.Meter[meterName].Handler.AddMeasurand(meterConfig.Measurand)
		for _, measurandName := range global.AllMeters.Meter[meterName].Handler.ListMeasurand() {
			global.AllMeters.Meter[meterName].Measurand[measurandName] = &global.Value{}
		}
	}

	return
}

func loop() {
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
		global.AllMeters.Time = time.Now()
	}()

	debug.InfoLog.Println("runtime to receive data: ", time.Since(runTime))
	runTime = time.Now()

	if err := WriteToCSV(&global.AllMeters, &global.Config); err != nil {
		debug.ErrorLog.Printf("writing to CSV file: %q", err)
	}
	if err := WriteToInflux(&global.AllMeters, &global.Config); err != nil {
		debug.ErrorLog.Printf("writing to influx db: %q", err)
	}
	if err := WriteToMQTT(&global.AllMeters, &global.Config); err != nil {
		debug.ErrorLog.Printf("sending to mqtt: %q", err)
	}
	debug.InfoLog.Println("runtime to write data: ", time.Since(runTime))
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
		case *float64:
			*x, _ = strconv.ParseFloat(value, 32)
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
	influxClient.Open(config.Influx.URL, fmt.Sprintf("%s:%s", config.Influx.User, config.Influx.Password), config.Influx.Database)
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
						var f float64
						getField(&t, cfgRecord, "type")

						switch t {
						case "value":
							f = measurand.Value
						case "delta":
							f = measurand.Delta
						case "avg":
							f = measurand.Avg
						}

						if f == 0 && func() bool {
							var e string
							getField(&e, cfgRecord, "exclude")
							return tools.In(e, "0")
						}() {
							continue
						}
						record[n] = f
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

type mqttMessage struct {
	Topic    string
	Payload  []byte
	Qos      byte
	Retained bool
}

type mqttPayload struct {
	TimeStamp        time.Time
	MeterReading     float64
	UnitMeterReading string
	Flow             float64
	UnitFlow         string
}

func WriteToMQTT(m *global.Meters, config *global.Configuration) (err error) {
	if config.Mqtt.URL == "" {
		return nil
	}

	for meterName, meter := range m.Meter {
		if met, ok := config.Meter[meterName]; !ok || met.Mqtt.Topic == "" {
			continue
		}

		message := mqttMessage{
			Topic:    config.Meter[meterName].Mqtt.Topic,
			Qos:      0,
			Retained: true,
		}
		payload := mqttPayload{
			TimeStamp: meter.Time,
		}
		for measurandName, measurand := range meter.Measurand {
			if cfgRecords, ok := config.Measurand[measurandName]; ok {

				for cfgName, cfgRecord := range cfgRecords {
					var out string
					var p string

					getField(&out, cfgRecord, "out")

					if !isIn("mqtt", out) {
						continue
					}

					getField(&p, cfgRecord, "type")
					if p != "value" {
						continue
					}
					switch cfgName {
					case "power (avg)", "power to grid(avg)", "power (w)":
						payload.Flow = measurand.Value
						payload.UnitFlow = "w"
					case "energy (wh)", "energy to grid (wh)":
						payload.MeterReading = measurand.Value
						payload.UnitMeterReading = "Wh"
					case "liter (avg)", "liter (l/h)":
						payload.Flow = measurand.Value
						payload.UnitFlow = "l/h"
					case "liter (l)":
						payload.MeterReading = measurand.Value
						payload.UnitMeterReading = "l"
					}
				}
			}
		}

		message.Payload, err = json.MarshalIndent(payload, "", "  ")
		if err != nil {
			debug.ErrorLog.Printf("sending mqtt: %q", err)
			return err
		}

		go func(msg mqttMessage) {
			opts := mqtt.NewClientOptions().AddBroker(config.Mqtt.URL)
			handler := mqtt.NewClient(opts)
			t := handler.Connect()
			defer handler.Disconnect(260)

			<-t.Done()
			if err := t.Error(); err != nil {
				debug.ErrorLog.Printf("sending mqtt: %q", err)
				return
			}

			debug.DebugLog.Printf("publishing %v bytes to topic %v", len(msg.Payload), msg.Topic)
			t = handler.Publish(msg.Topic, msg.Qos, msg.Retained, msg.Payload)

			// the asynchronous nature of this library makes it easy to forget to check for errors.
			// Consider using a go routine to log these
			go func() {
				<-t.Done()
				if err := t.Error(); err != nil {
					debug.ErrorLog.Printf("publishing topic %v: %v", msg.Topic, err)
				}
			}()
		}(message)

	}

	return
}
