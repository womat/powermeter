package main

import (
	"fmt"
	"path"
	"powermeter/pkg/csv"
	"time"

	"powermeter/global"
	_ "powermeter/pkg/config"
	"powermeter/pkg/debug"
	"powermeter/pkg/energy"
	"powermeter/pkg/fritz"
	"powermeter/pkg/mbclient"
	"powermeter/pkg/mbgw"
)

const Delta = "delta"

type Value struct {
	from      time.Time
	to        time.Time
	value     float64
	delta     float64
	lastValue float64
	lastTime  time.Time
}

type Meter struct {
	Measurand map[string]Value
	Meter     energy.Meter
}

func main() {
	Meters := map[string]Meter{}

	for name, meter := range global.Config.Meter {
		switch t := meter.Type; t {
		case "mbclient":
			c := mbclient.NewClient()
			if err := c.Listen(meter.Connection); err != nil {
				debug.ErrorLog.Printf("error to start modbus client %v: %v\n", meter.Connection, err)
				return
			}

			Meters[name] = Meter{Meter: c, Measurand: map[string]Value{}}
		case "mbgateway":
			c := mbgw.NewClient()
			if err := c.Listen(meter.Connection); err != nil {
				debug.ErrorLog.Printf("error to start modbus gateway client %v: %v\n", meter.Connection, err)
				return
			}

			Meters[name] = Meter{Meter: c, Measurand: map[string]Value{}}
		case "fritz!powerline":
			c := fritz.NewClient()
			if err := c.Listen(meter.Connection); err != nil {
				debug.ErrorLog.Printf("error to start fritz!powerline client %v: %v\n", meter.Connection, err)
				return
			}
			Meters[name] = Meter{Meter: c, Measurand: map[string]Value{}}
		default:
			debug.WarningLog.Printf("client type %v is not supported\n", t)
		}

		Meters[name].Meter.AddMeasurand(meter.Measurand)
	}

	// TODO: Garbage Collector (cleanup HistoryEnergy)

	ticker := time.NewTicker(global.Config.TimerPeriod)
	defer ticker.Stop()

	for {
		t := time.Now()
		for name, meter := range Meters {
			for _, measurandName := range meter.Meter.ListMeasurand() {
				mparam, ok := global.Config.Measurand[measurandName]
				if !ok {
					continue
				}

				if _, ok := meter.Measurand[measurandName]; !ok {
					meter.Measurand[measurandName] = Value{}
				}

				tempValue := meter.Measurand[measurandName]
				t := time.Now()
				v, _ := meter.Meter.GetMeteredValue(measurandName)

				if t.Sub(tempValue.lastTime).Hours() > 24*365*10 {
					tempValue.lastTime = t
				}

				if mparam.Type == Delta && v == 0 {
					v = tempValue.lastValue
				}

				tempValue.to = t
				tempValue.value = v
				tempValue.from = tempValue.lastTime

				if tempValue.lastValue > 0 && mparam.Type == Delta {

					tempValue.delta = (v - tempValue.lastValue) / t.Sub(tempValue.lastTime).Hours()
				}

				tempValue.lastValue = v
				tempValue.lastTime = t
				meter.Measurand[measurandName] = tempValue
			}
			Meters[name] = meter
		}

		record := map[string]interface{}{}
		for name, meter := range Meters {
			for measurandName, measurand := range meter.Measurand {
				headerName := name + "-" + measurandName
				record["From"] = measurand.from
				record["To"] = measurand.to
				record[headerName] = measurand.value
				if m, ok := global.Config.Measurand[measurandName]; ok && m.Type == Delta {
					record[headerName+"-delta"] = measurand.delta
				}
			}
		}

		var err error
		func() {
			csvFileName := path.Join(global.Config.Csv.Path, csv.FileName(global.Config.Csv.FilenameFormat, time.Now()))
			csvWriter := csv.New()
			csvWriter.ValueSeparator = rune(global.Config.Csv.Separator[0])
			csvWriter.DecimalSeparator = rune(global.Config.Csv.DecimalSeparator[0])
			if err = csvWriter.Open(csvFileName); err != nil {
				debug.ErrorLog.Printf("error open file %v: %v", csvFileName, err)
				err = fmt.Errorf("error open file %v: %v", csvFileName, err)
				return
			}
			defer csvWriter.Close()

			if csvWriter.IsNewFile() {
				if err = csvWriter.WriteOnlyHeader(record); err != nil {
					err = fmt.Errorf("error write header %v: %v", record, err)
					return
				}
			}

			var r []map[string]interface{}
			r = append(r, record)

			if err = csvWriter.Write(r); err != nil {
				err = fmt.Errorf("error write csv file: %v", err)
				return
			}
			return
		}()

		if err != nil {
			debug.ErrorLog.Println(err)
		}
		debug.InfoLog.Println("runtime: ", time.Since(t))
		<-ticker.C
	}
}
