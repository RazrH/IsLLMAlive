package main

import (
	"context"
	"flag"
	"github.com/getlantern/systray"
	"isllmalive/internal/diagnostics"
	"log"
	"sync"
	"time"
)

var metricsFile string
var metricsInterval time.Duration
var runFor time.Duration
var measurementMu sync.Mutex
var measurementCancel context.CancelFunc
var measurementDone chan struct{}
var measurementTimer *time.Timer
var measurementsStopped bool

func init() {
	flag.StringVar(&metricsFile, "metrics-file", "", "explicit local JSONL runtime diagnostics file (must not exist)")
	flag.DurationVar(&metricsInterval, "metrics-interval", 30*time.Second, "runtime sampling interval")
	flag.DurationVar(&runFor, "run-for", 0, "optional measurement run duration; zero runs until normal exit")
}
func startMeasurements() {
	measurementMu.Lock()
	defer measurementMu.Unlock()
	if measurementsStopped {
		return
	}
	if metricsFile != "" {
		ctx, cancel := context.WithCancel(context.Background())
		measurementCancel = cancel
		measurementDone = make(chan struct{})
		go func() {
			defer close(measurementDone)
			if err := diagnostics.Sample(ctx, metricsFile, metricsInterval, mainApp.Diagnostics); err != nil {
				log.Printf("Runtime sampling failed: %v", err)
			}
		}()
	}
	if runFor > 0 {
		measurementTimer = time.AfterFunc(runFor, systray.Quit)
	}
}
func stopMeasurements() {
	measurementMu.Lock()
	measurementsStopped = true
	if measurementTimer != nil {
		measurementTimer.Stop()
	}
	if measurementCancel != nil {
		measurementCancel()
	}
	done := measurementDone
	measurementMu.Unlock()
	if done != nil {
		<-done
	}
}
