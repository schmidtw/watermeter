// Package watermeter provide simple implementation of a water meter
package watermeter

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

type entry struct {
	time  time.Time
	total uint64
}

// A Watermeter represents a watermeter with a simple magnet and sensor set
// at a specific volume flow rate.
type Watermeter struct {
	Timeout time.Duration
	Usage   func(gallons uint64, flow float64)
	Change  func()

	now        func() time.Time
	lastGallon entry
	total      uint64
	events     list.List
	mutex      sync.Mutex
}

func (e *entry) String() string {
	return fmt.Sprintf("time: %s, total: %d", e.time, e.total)
}

// String returns the formatted string representation of the object.
func (w *Watermeter) String() string {
	rv := fmt.Sprintf("{\n\tTimeout: %s,\n\tUsage: %p,\n\tChange: %p,\n\tnow: %p,\n\tlastGallon{ %s },\n\ttotal: %d,\n\tevents { ", w.Timeout, w.Usage, w.Change, w.now, w.lastGallon.String(), w.total)
	e := w.events.Front()
	comma := ""
	for nil != e {
		rv += fmt.Sprintf("%s\n\t\t{ %s }", comma, e.Value.(*entry).String())
		comma = ","
		e = e.Next()
	}
	rv += fmt.Sprintf("\n\t}\n}")

	return rv
}

// Init initializes the watermeter object to the initial state.
// Argument initial is the initial running total in 1/1000 gallon units.
func (w *Watermeter) Init(initial uint64) *Watermeter {

	if nil == w.now {
		w.now = func() time.Time { return time.Now() }
	}

	w.total = initial
	w.mutex = sync.Mutex{}
	w.events.Init()

	e := new(entry)
	e.time = w.now()
	e.total = w.total
	w.events.PushFront(e)
	w.lastGallon = *e

	return w
}

// GetFlow gets the flow rate (gallons/min) over the specified duration.
func (w *Watermeter) GetFlow(duration time.Duration) float64 {
	now := w.now()
	then := now.Add(-duration)

	end := entry{time: now, total: w.total}
	start := entry{time: now, total: w.total}

	w.mutex.Lock()

	item := w.events.Front()

	for nil != item {
		e := item.Value.(*entry)
		if then.Equal(e.time) || then.Before(e.time) {
			start.time = e.time
			start.total = e.total
			item = item.Next()
		} else {
			item = nil
		}
	}
	w.mutex.Unlock()

	volumeDelta := end.total - start.total
	return float64(volumeDelta) / 1000 / duration.Minutes()
}

// GetGallons gets the gallon running count.
func (w *Watermeter) GetGallons() uint64 {
	return w.total / 1000
}

// Update updates the watermeter with the specified number of 1/1000 gallons
// that have passed through the meter.
func (w *Watermeter) Update(mGallons uint) {
	now := w.now()
	prune := now.Add(-w.Timeout)

	w.mutex.Lock()
	before := w.total / 1000
	w.total += uint64(mGallons)
	after := w.total / 1000

	e := new(entry)
	e.time = now
	e.total = w.total
	w.events.PushFront(e)

	done := false
	for false == done {
		item := w.events.Back()
		e := item.Value.(*entry)
		if e.time.Before(prune) {
			w.events.Remove(item)
		} else {
			done = true
		}
		if 3 > w.events.Len() {
			done = true
		}
	}

	w.mutex.Unlock()

	if nil != w.Change {
		go (w.Change)()
	}

	if (after - before) > 0 {
		if nil != w.Usage {
			flow := float64(e.total-w.lastGallon.total) / 1000
			flow /= e.time.Sub(w.lastGallon.time).Minutes()
			go (w.Usage)(after, flow)
		}
		w.lastGallon = *e
	}
}
