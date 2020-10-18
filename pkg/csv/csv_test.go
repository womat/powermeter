package csv

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"powermeter/pkg/tools"
	"testing"
	"time"
)

func TestFloat2string(t *testing.T) {

	type test struct {
		f                float64
		expect           string
		decimalSeparator rune
	}

	sequence := []test{
		{f: 1.2, decimalSeparator: ':', expect: "1:200000"},
		{f: 192, decimalSeparator: '?', expect: "192?000000"},
		{f: 0.04, decimalSeparator: ',', expect: "0,040000"},
		{f: 1000000000, decimalSeparator: '*', expect: "1000000000*000000"}}

	for _, i := range sequence {
		got := float2string(i.decimalSeparator, i.f)
		expect := i.expect
		if !tools.IsEqual(expect, got) {
			t.Errorf("expected %v, got %v", expect, got)
		}
	}
	for _, i := range sequence {
		got := float2string(i.decimalSeparator, float32(i.f))
		expect := i.expect
		if !tools.IsEqual(expect, got) {
			t.Errorf("expected %v, got %v", expect, got)
		}
	}
}

func TestDate2string(t *testing.T) {

	type test struct {
		t          time.Time
		expect     string
		dateFormat string
	}

	sequence := []test{
		{t: time.Date(2021, time.November, 10, 23, 22, 33, 0, time.UTC), dateFormat: "yyyy-mm-dd HH:MM:SS", expect: "2021-11-10 23:22:33"},
		{t: time.Date(2021, time.November, 10, 23, 22, 33, 0, time.UTC), dateFormat: "yyyy-mm-dd HH:MM", expect: "2021-11-10 23:22"},
		{t: time.Date(2021, time.November, 10, 23, 21, 34, 0, time.UTC), dateFormat: "yymmdd HH:MM:SS", expect: "211110 23:21:34"},
		{t: time.Date(2021, time.November, 10, 23, 0, 12, 0, time.UTC), dateFormat: "HH:MM:SS", expect: "23:00:12"},
		{t: time.Date(2021, time.October, 11, 23, 0, 0, 0, time.UTC), dateFormat: "dd-mm-yyyy", expect: "11-10-2021"}}

	for _, i := range sequence {
		got := date2string(i.dateFormat, i.t)
		expect := i.expect
		if !tools.IsEqual(expect, got) {
			t.Errorf("expected %v, got %v", expect, got)
		}
	}
}

func TestCSVOrg(t *testing.T) {

	fileName := filepath.Join("C:\\temp", FileName("Energy_yyyymm.csv", time.Now()))
	fileAlreadyExists := fileExists(fileName)
	csvFile, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return
	}
	defer csvFile.Close()

	records := [][]string{}

	if !fileAlreadyExists {
		records = append(records, []string{"time", "S1", "S2"})
	}
	records = append(records, []string{date2string("yyyy-mm-dd HH:MM:SS", time.Now()), "1", "2"})
	records = append(records, []string{date2string("yyyy-mm-dd HH:MM:SS", time.Now()), "2", "b"})

	w := csv.NewWriter(csvFile)
	w.Comma = ';'
	w.WriteAll(records) // calls Flush internally

	if err := w.Error(); err != nil {
		log.Fatalln("error writing csv:", err)
	}
	return
}

func TestCSV(t *testing.T) {
	const fileName = "c:\\temp\\test.csv"

	if fileExists(fileName) {
		if err := os.Remove(fileName); err != nil {
			t.Fatalf("error to delete file: %v\n", err)
		}
	}

	c := New()
	if err := c.Open(fileName); err != nil {
		t.Fatalf("error to open file: %v\n", err)
	}

	records := []map[string]interface{}{
		{"col1": "r11", "col2": "r12", "col3": "r13", "col4": "14"},
		{"col1": "r21", "col2": "r22", "col3": "r23", "col4": "24"}}

	if err := c.WriteHeader(records); err != nil {
		t.Fatalf("error write csv data %v\n", err)

	}

	records = []map[string]interface{}{
		{"col1": 11, "col2": 12, "col3": 13, "col4": 14},
		{"col1": 2.1, "col2": 2.2, "col3": 2.3, "col4": 2.4}}

	if err := c.Write(records); err != nil {
		t.Fatalf("error write csv data %v\n", err)

	}

	c.Close()

	c = New()
	if err := c.Open(fileName); err != nil {
		t.Fatalf("error to open file: %v\n", err)
	}
	records = []map[string]interface{}{
		{"col1": 11, "col2": 12, "col3": 13, "col4": 14},
		{"col1": 2.1, "col2": 2.2, "col3": 2.3, "col5": 2.4}}

	if err := c.Write(records); err != nil {
		t.Fatalf("error write csv data %v\n", err)

	}
	c.Close()

}
