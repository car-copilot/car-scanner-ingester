package obd2influx

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

type CarDataPoint struct {
	time  time.Time
	pid   string
	value float64
	unit  string
}

func (p *CarDataPoint) ToInfluxPoint() *write.Point {
	return write.NewPoint(
		p.pid,
		map[string]string{
			"unit": p.unit,
		},
		map[string]interface{}{
			"value": p.value,
		},
		p.time,
	)
}

func recordToCarDatapoint(record []string, start_time time.Time, offset *float64) CarDataPoint {
	value, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		panic(err)
	}
	seconds, err := strconv.ParseFloat(record[0], 64)
	if err != nil {
		panic(err)
	}
	if *offset > 0.0 {
		start_time = start_time.Add(time.Duration((seconds - *offset) * float64(time.Second)))
	} else {
		*offset = seconds
	}
	return CarDataPoint{
		time:  start_time,
		pid:   record[1],
		value: value,
		unit:  record[3],
	}
}

func ReadCsv(path string) []*write.Point {
	out := []*write.Point{}
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	start := time.Now()
	second_offset := 0.0
	r := csv.NewReader(file)
	r.Comma = ';'

	records, err := r.ReadAll()
	if err != nil {
		panic(err)
	}
	records = records[1:] // remove head
	for _, record := range records {
		point := recordToCarDatapoint(record, start, &second_offset)
		out = append(out, point.ToInfluxPoint())
		fmt.Print(point)
	}
	return out
}
