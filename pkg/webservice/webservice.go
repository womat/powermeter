package webservice

import (
	"encoding/json"
	"net/http"
	"strconv"

	"powermeter/global"
)

func init() {
	InitWebService()
}

func InitWebService() (err error) {
	for pattern, f := range map[string]func(http.ResponseWriter, *http.Request){
		"version":     httpGetVersion,
		"currentdata": httpReadCurrentData,
	} {
		if set, ok := global.Config.Webserver.Webservices[pattern]; ok && set {
			http.HandleFunc("/"+pattern, f)
		}
	}

	if set, ok := global.Config.Webserver.Webservices["meter"]; ok && set {
		for pattern := range global.Config.Meter {
			http.HandleFunc("/"+pattern, httpReadMeterData)
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

// httpReadMeterData supplies the data of al meters
func httpReadMeterData(w http.ResponseWriter, r *http.Request) {
	var j []byte
	var err error

	pattern := r.RequestURI[1:]
	meter, ok := global.AllMeters.Meter[pattern]
	if !ok {
		errorLog.Printf("invalid meter: %q\n", pattern)
		return
	}

	func() {
		meter.RLock()
		defer meter.RUnlock()
		j, err = json.MarshalIndent(&meter, "", "  ")
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
