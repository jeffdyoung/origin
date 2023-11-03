package monitorserialization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Event is not an interval.  It is an instant.  The instant removes any ambiguity about "when"
type EventInterval struct {
	Level string `json:"level"`

	Locator string `json:"locator"`
	Message string `json:"message"`

	// TODO: Remove the omitempty, just here to keep from having to repeatedly updated the json
	// files used in some new tests
	Source    string `json:"tempSource,omitempty"`    // also temporary, unsure if this concept will survive
	SubSource string `json:"tempSubSource,omitempty"` // also temporary, unsure if this concept will survive

	// TODO: we're hoping to move these to just locator/message when everything is ready.
	StructuredLocator monitorapi.Locator `json:"tempStructuredLocator"`
	StructuredMessage monitorapi.Message `json:"tempStructuredMessage"`

	From metav1.Time `json:"from"`
	To   metav1.Time `json:"to"`
}

// EventList is not an interval.  It is an instant.  The instant removes any ambiguity about "when"
type EventIntervalList struct {
	Items []EventInterval `json:"items"`
}

func EventsToFile(filename string, events monitorapi.Intervals) error {
	json, err := IntervalsToJSON(events)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, json, 0644)
}

func EventsFromFile(filename string) (monitorapi.Intervals, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return IntervalsFromJSON(data)
}

func IntervalsFromJSON(data []byte) (monitorapi.Intervals, error) {
	var list EventIntervalList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	events := make(monitorapi.Intervals, 0, len(list.Items))
	for _, interval := range list.Items {
		level, err := monitorapi.ConditionLevelFromString(interval.Level)
		if err != nil {
			return nil, err
		}
		events = append(events, monitorapi.Interval{
			Source: monitorapi.IntervalSource(interval.Source),
			Condition: monitorapi.Condition{
				Level:             level,
				Locator:           interval.Locator,
				StructuredLocator: interval.StructuredLocator,
				Message:           interval.Message,
				StructuredMessage: interval.StructuredMessage,
			},

			From: interval.From.Time,
			To:   interval.To.Time,
		})
	}

	return events, nil
}

func IntervalFromJSON(data []byte) (*monitorapi.Interval, error) {
	var serializedInterval EventInterval
	if err := json.Unmarshal(data, &serializedInterval); err != nil {
		return nil, err
	}
	level, err := monitorapi.ConditionLevelFromString(serializedInterval.Level)
	if err != nil {
		return nil, err
	}
	return &monitorapi.Interval{
		Source: monitorapi.IntervalSource(serializedInterval.Source),
		Condition: monitorapi.Condition{
			Level:             level,
			Locator:           serializedInterval.Locator,
			Message:           serializedInterval.Message,
			StructuredLocator: serializedInterval.StructuredLocator,
			StructuredMessage: serializedInterval.StructuredMessage,
		},

		From: serializedInterval.From.Time,
		To:   serializedInterval.To.Time,
	}, nil
}

func IntervalToOneLineJSON(interval monitorapi.Interval) ([]byte, error) {
	outputEvent := monitorEventIntervalToEventInterval(interval)

	spacedBytes, err := json.Marshal(outputEvent)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if err := json.Compact(buf, spacedBytes); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func IntervalsToJSON(intervals monitorapi.Intervals) ([]byte, error) {
	outputEvents := []EventInterval{}
	for _, curr := range intervals {
		outputEvents = append(outputEvents, monitorEventIntervalToEventInterval(curr))
	}

	sort.Sort(byTime(outputEvents))
	list := EventIntervalList{Items: outputEvents}
	return json.MarshalIndent(list, "", "    ")
}

func IntervalsToFile(filename string, intervals monitorapi.Intervals) error {
	json, err := EventsIntervalsToJSON(intervals)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, json, 0644)
}

// TODO: this is very similar but subtly different to the function above, what is the purpose of skipping those
// with from/to equal or empty to?
func EventsIntervalsToJSON(events monitorapi.Intervals) ([]byte, error) {
	outputEvents := []EventInterval{}
	for _, curr := range events {
		if curr.From == curr.To && !curr.To.IsZero() {
			continue
		}

		outputEvents = append(outputEvents, monitorEventIntervalToEventInterval(curr))
	}

	sort.Sort(byTime(outputEvents))
	list := EventIntervalList{Items: outputEvents}
	return json.MarshalIndent(list, "", "    ")
}

func monitorEventIntervalToEventInterval(interval monitorapi.Interval) EventInterval {
	ret := EventInterval{
		Level:             fmt.Sprintf("%v", interval.Level),
		Locator:           interval.Locator,
		Message:           interval.Message,
		StructuredLocator: interval.StructuredLocator,
		StructuredMessage: interval.StructuredMessage,
		Source:            string(interval.Source),

		From: metav1.Time{Time: interval.From},
		To:   metav1.Time{Time: interval.To},
	}
	return ret
}

type byTime []EventInterval

func (intervals byTime) Less(i, j int) bool {
	// currently synced with https://github.com/openshift/origin/blob/9b001745ec8006eb406bd92e3555d1070b9b656e/pkg/monitor/monitorapi/types.go#L425

	switch d := intervals[i].From.Sub(intervals[j].From.Time); {
	case d < 0:
		return true
	case d > 0:
		return false
	}

	switch d := intervals[i].To.Sub(intervals[j].To.Time); {
	case d < 0:
		return true
	case d > 0:
		return false
	}
	if intervals[i].Message != intervals[j].Message {
		return intervals[i].Message < intervals[j].Message
	}

	return intervals[i].Locator < intervals[j].Locator
}

func (intervals byTime) Len() int { return len(intervals) }
func (intervals byTime) Swap(i, j int) {
	intervals[i], intervals[j] = intervals[j], intervals[i]
}
