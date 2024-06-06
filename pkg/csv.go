package obd2influx

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/bcicen/go-units"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/rs/zerolog/log"
)

func unitConversion(unit string, value float64) (newUnit string, newValue float64) {
	switch unit {
	case "bar":
		newUnit = "hPa"
		newValue = value * 100
	default:
		newUnit = unit
		newValue = value
	}

	return
}

type CarDataPoint struct {
	Time      time.Time
	Pid       string
	Value     float64
	Unit      string
	Latitude  float64
	Longitude float64
}

func (p CarDataPoint) ToInfluxPoint() *write.Point {
	return write.NewPoint(
		p.Pid,
		map[string]string{
			"unit": p.Unit,
		},
		map[string]interface{}{
			"value": p.Value,
			"lat":   p.Latitude,
			"lon":   p.Longitude,
		},
		p.Time,
	)
}

func contains(slice []string, element string) bool {
	for _, e := range slice {
		if e == element {
			return true
		}
	}
	return false
}

func recordToCarDatapoint(record []string, start_time time.Time, offset *float64, mapping map[string]string, ignorePids []string, convert map[string]Convetion) (CarDataPoint, error) {
	value, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		panic(err)
	}

	latitute, err := strconv.ParseFloat(record[4], 64)
	if err != nil {
		latitute = 0.0
	}

	longitude, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		longitude = 0.0
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

	pid := record[1]

	if ok := contains(ignorePids, pid); ok {
		return CarDataPoint{}, fmt.Errorf("ignoring PID %s", pid)
	}

	if conv, ok := convert[pid]; ok {
		from, err := units.Find(conv.From)
		if err != nil {
			return CarDataPoint{}, fmt.Errorf("error finding unit %s", conv.From)
		}
		to, err := units.Find(conv.To)
		if err != nil {
			return CarDataPoint{}, fmt.Errorf("error finding unit %s", conv.To)
		}
		valueRes, err := units.ConvertFloat(value, from, to)
		value = valueRes.Float()
		if err != nil {
			return CarDataPoint{}, fmt.Errorf("error converting value %f from %s to %s", value, conv.From, conv.To)
		}
	}

	if mapped, ok := mapping[pid]; ok {
		pid = mapped
	}

	unit, value := unitConversion(record[3], value)

	return CarDataPoint{
		Time:      start_time,
		Pid:       pid,
		Value:     value,
		Unit:      unit,
		Latitude:  latitute,
		Longitude: longitude,
	}, nil
}

func GroupDataPoint(data []CarDataPoint, group_size time.Duration) []CarDataPoint {
	groups := make(map[time.Time]map[string]CarDataPoint)
	for _, point := range data {
		group_time := point.Time.Truncate(group_size)
		if _, ok := groups[group_time]; !ok {
			groups[group_time] = map[string]CarDataPoint{}
		}
		point.Time = group_time
		groups[group_time][point.Pid] = point
	}

	flat := []CarDataPoint{}
	for _, group := range groups {
		for _, point := range group {
			flat = append(flat, point)
		}
	}
	return flat
}

func ReadCsv(data io.Reader, date time.Time, mapping map[string]string, ignorePids []string, convert map[string]Convetion) ([]CarDataPoint, time.Time) {
	out := []CarDataPoint{}
	end := date

	start := date
	second_offset := 0.0
	r := csv.NewReader(data)
	r.Comma = ';'

	records, err := r.ReadAll()
	if err != nil {
		panic(err)
	}

	records = records[1:] // remove head
	for _, record := range records {
		point, err := recordToCarDatapoint(record, start, &second_offset, mapping, ignorePids, convert)
		if err != nil {
			log.Warn().Err(err)
			continue
		}
		// update end date
		if point.Time.After(end) {
			end = point.Time
		}

		out = append(out, point)
		// fmt.Print(point)
	}

	// max time interval

	return out, end
}

func findMaxTimeInterval(out []CarDataPoint) time.Duration {
	var previous CarDataPoint
	var current CarDataPoint
	var interval time.Duration
	for _, point := range out {
		if point.Pid == "Vehicle speed" {
			if previous == (CarDataPoint{}) {
				previous = point
			} else {
				current = point
				if interval == 0 {
					log.Debug().Msgf("First time interval between %s and %s:  %s", current.Time, previous.Time, current.Time.Sub(previous.Time).Abs())
					interval = current.Time.Sub(previous.Time).Abs()
				} else {
					if current.Time.Sub(previous.Time).Abs() > interval {
						log.Debug().Msgf("New max time interval: %s", current.Time.Sub(previous.Time).Abs())
						interval = current.Time.Sub(previous.Time).Abs()
					}
				}
				previous = current
			}

		}
	}
	return interval
}

func findMeanTimeInterval(out []CarDataPoint) time.Duration {
	var previous CarDataPoint
	var current CarDataPoint
	var interval time.Duration
	var sum time.Duration
	var count int
	for _, point := range out {
		if point.Pid == "Vehicle speed" {
			if previous == (CarDataPoint{}) {
				previous = point
			} else {
				current = point
				if interval == 0 {
					log.Debug().Msgf("First time interval between %s and %s:  %s", current.Time, previous.Time, current.Time.Sub(previous.Time).Abs())
					interval = current.Time.Sub(previous.Time).Abs()
				} else {
					sum += current.Time.Sub(previous.Time).Abs()
					count++
				}
				previous = current
			}

		}
	}
	return sum / time.Duration(count)
}
