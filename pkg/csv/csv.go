package csv

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vigneshuvi/GoDateFormat"
)

const (
	LF                      = "\n"
	CRLF                    = "\n"
	defaultValueSeparator   = ';'
	defaultDecimalSeparator = ','
	defaultRowSeparator     = LF
	defaultDateFormat       = "yyyy-mm-dd HH:MM:SS"
)

var (
	errHeaderDoesntMatch     = errors.New("data doesn't match with header")
	errorHeaderAlreadyExists = errors.New("header already exists")
)

type Writer struct {
	ValueSeparator, DecimalSeparator rune
	RowSeparator, DateFormat         string
	header                           []string
	isNewFile                        bool
	fileName                         string
	file                             *os.File
	writer                           *csv.Writer
}

func New() *Writer {
	return &Writer{
		ValueSeparator:   defaultValueSeparator,
		DecimalSeparator: defaultDecimalSeparator,
		RowSeparator:     defaultRowSeparator,
		DateFormat:       defaultDateFormat,
	}
}

func (c *Writer) Open(fileName string) (err error) {
	//	fileName := filepath.Join("C:\\temp", FileName("Energy_yyyymm.csv", time.Now()))
	c.fileName = fileName
	c.isNewFile = !fileExists(c.fileName)
	if c.file, err = os.OpenFile(c.fileName, os.O_APPEND|os.O_CREATE|os.O_RDWR, os.ModePerm); err != nil {
		return
	}

	if err := c.getHeader(); err != nil && err != io.EOF {
		return err
	}
	c.writer = csv.NewWriter(c.file)
	return
}

func (c *Writer) Close() error {
	return c.file.Close()
}

func (c *Writer) WriteHeader(records []map[string]interface{}) (err error) {
	if err = c.WriteOnlyHeader(records[0]); err != nil && err != errorHeaderAlreadyExists {
		return err
	}
	return c.write(records)
}

func (c *Writer) Write(records []map[string]interface{}) (err error) {
	return c.write(records)
}

func (c *Writer) WriteOnlyHeader(header map[string]interface{}) (err error) {
	if c.header != nil {
		return errorHeaderAlreadyExists
	}

	csvRecords := make([][]string, 0, 1)
	csvHeader := make([]string, 0, len(header))
	c.writer.Comma = c.ValueSeparator
	c.writer.UseCRLF = c.RowSeparator == CRLF

	for name := range header {
		csvHeader = append(csvHeader, name)
	}
	sort.Strings(csvHeader)
	csvRecords = append(csvRecords, csvHeader)

	if err = c.writer.WriteAll(csvRecords); err != nil {
		errorLog.Println("error writing header:", err)
		return
	}

	c.header = csvHeader
	debugLog.Printf("Filename: %s written header: %v\n", c.fileName, csvRecords)
	return
}

func (c *Writer) write(records []map[string]interface{}) (err error) {
	c.writer.Comma = c.ValueSeparator
	c.writer.UseCRLF = c.RowSeparator == CRLF
	csvRecords := make([][]string, 0, len(records))

	for _, row := range records {
		csvHeader := make([]string, 0, len(row))
		for name := range row {
			csvHeader = append(csvHeader, name)
		}
		sort.Strings(csvHeader)
		if c.header == nil {
			c.header = csvHeader
		}

		if !isEqual(c.header, csvHeader) {
			return errHeaderDoesntMatch
		}

		csvRow := make([]string, 0, len(csvHeader))
		for _, column := range csvHeader {
			var valueString string
			switch v := row[column].(type) {
			case time.Time:
				valueString = date2string(c.DateFormat, v)
			case string:
				valueString = v
			case int:
				valueString = strconv.Itoa(v)
			case float32, float64:
				valueString = float2string(c.DecimalSeparator, v)
			}
			csvRow = append(csvRow, valueString)
		}

		csvRecords = append(csvRecords, csvRow)
	}
	// calls Flush internally
	if err = c.writer.WriteAll(csvRecords); err != nil {
		errorLog.Println("error writing csv:", err)
		return
	}

	debugLog.Printf("Filename: %s written records: %v\n", c.fileName, len(csvRecords))
	return
}

func (c *Writer) IsNewFile() bool {
	return c.isNewFile
}

//FileName create a formatted csv output file
func FileName(fmt string, t time.Time) (fileName string) {
	fileName = t.Format(GoDateFormat.ConvertFormat(fmt))
	return fileName
}

func fileExists(fileName string) bool {
	info, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func date2string(fmt string, t time.Time) (s string) {
	s = t.Format(GoDateFormat.ConvertFormat(fmt))
	return
}

func float2string(decimalSeparator rune, f interface{}) (s string) {
	s = fmt.Sprintf("%f", f)
	s = strings.Replace(s, ".", string(decimalSeparator), -1)
	return
}

func (c *Writer) getHeader() (err error) {
	if c.isNewFile {
		return
	}

	var header []string
	reader := csv.NewReader(c.file)
	reader.Comma = c.ValueSeparator

	if header, err = reader.Read(); err != nil {
		return
	}

	sort.Strings(header)
	c.header = header
	return
}

func isEqual(a interface{}, b interface{}) bool {
	expect, _ := json.Marshal(a)
	got, _ := json.Marshal(b)
	return string(expect) == string(got)
}

func mapKey(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func mapValue(m map[string]interface{}, v []interface{}) {
	keys := mapKey(m)
	values := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		if v, ok := m[key]; ok {
			values = append(values, v)
		}
	}
}
