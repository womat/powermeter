package webservice

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"powermeter/global"
	"powermeter/pkg/debug"
)

type httpData struct {
	Time      time.Time
	RunTime   float64
	Measurand map[string]float64
}

func init() {
	InitWebService()
}

func InitWebService() (err error) {
	for pattern, f := range map[string]func(http.ResponseWriter, *http.Request){
		"version":     httpGetVersion,
		"currentdata": httpReadCurrentData,
		"allmeters":   httpReadMeters,
	} {
		if set, ok := global.Config.Webserver.Webservices[pattern]; ok && set {
			http.HandleFunc("/"+pattern, f)
		}
	}

	if set, ok := global.Config.Webserver.Webservices["meter"]; ok && set {
		for pattern := range global.Config.Meter {
			http.HandleFunc("/"+pattern, httpReadMeter)
		}
	}

	port := ":" + strconv.Itoa(global.Config.Webserver.Port)
	go http.ListenAndServe(port, nil)
	return
}

// httpGetVersion prints the SW Version
func httpGetVersion(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(global.VERSION)); err != nil {
		errorLog.Println(err)
		return
	}
}

// httpReadMeter supplies the data of defined meters
func httpReadMeter(w http.ResponseWriter, r *http.Request) {
	var j []byte
	var err error

	meterName := r.RequestURI[1:]
	meter, ok := global.AllMeters.Meter[meterName]
	if !ok {
		errorLog.Printf("invalid meter: %q\n", meterName)
		return
	}

	t := time.Now()
	m, err := meter.Reader()
	if err != nil {
		debug.ErrorLog.Printf("error read values from meter %q: %v\n", meterName, err)
		return
	}

	data := httpData{
		Time:      time.Now(),
		RunTime:   time.Since(t).Seconds(),
		Measurand: m,
	}

	if j, err = json.MarshalIndent(data, "", "  "); err != nil {
		errorLog.Println(err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(j); err != nil {
		errorLog.Println(err)
		return
	}
}

// httpReadMeters supplies the data of all meters
func httpReadMeters(w http.ResponseWriter, r *http.Request) {
	var j []byte
	var err error

	data := map[string]httpData{}

	for meterName, meter := range global.AllMeters.Meter {
		t := time.Now()
		m, err := meter.Reader()
		if err != nil {
			debug.ErrorLog.Printf("error read values from meter %q: %v\n", meterName, err)
			return
		}

		data[meterName] = httpData{
			Time:      time.Now(),
			RunTime:   time.Since(t).Seconds(),
			Measurand: m,
		}
	}

	if j, err = json.MarshalIndent(data, "", "  "); err != nil {
		errorLog.Println(err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(j); err != nil {
		errorLog.Println(err)
		return
	}
}

// httpReadCurrentData supplies the data of al meters
func httpReadCurrentData(w http.ResponseWriter, r *http.Request) {
	var j []byte
	var err error

	func() {
		global.AllMeters.RLock()
		defer global.AllMeters.RUnlock()
		j, err = json.MarshalIndent(&global.AllMeters, "", "  ")
	}()

	if err != nil {
		errorLog.Println(err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(j); err != nil {
		errorLog.Println(err)
		return
	}
}
