package sensor

import (
	"time"
	"fmt"

	"github.com/canaryio/canary/pkg/sampler"
)

// Measurement reprents an aggregate of Target, Sample and error.
type Measurement struct {
	Target sampler.Target
	Sample sampler.Sample
	Error  error
}

// Sensor is capable of repeatedly measuring a given Target
// with a specific Sampler, and returns those results over channel C.
type Sensor struct {
	Target   sampler.Target
	C        chan Measurement
	Sampler  sampler.Sampler
	stopChan chan int
}

// take a sample against a target.
func (s *Sensor) measure() Measurement {
	sample, err := s.Sampler.Sample(s.Target)
	return Measurement{
		Target: s.Target,
		Sample: sample,
		Error:  err,
	}
}

// Start is meant to be called within a goroutine, and fires up the main event loop.
// interval is number of seconds. delay is number of ms.
func (s *Sensor) Start(interval int, delay float64) {
	fmt.Println("Start of sensor.Start() for: " + s.Target.URL)
	if s.stopChan == nil {
		s.stopChan = make(chan int)
	}

	// Delay for loop start offset.
	time.Sleep((time.Millisecond * time.Duration(delay)))

	// Start the ticker for this sensors interval
	//t := time.NewTicker((time.Second * time.Duration(interval)))

	// Measure, then wait for ticker interval
	s.C <- s.measure()

	for {
		fmt.Println("yay for loop")
		select {
		case <-s.stopChan:
			fmt.Println("We got a stopChan message in sensor: " + s.Target.URL)
			return
		// case <-t.C:
		// 	fmt.Println("We got a ticker tick in sensor: " + s.Target.URL)
		// 	s.C <- s.measure()
		}
	}
}

// Stop halts the event loop.
func (s *Sensor) Stop() {
	fmt.Println("Within stop() for sensor: " + s.Target.URL)
	s.stopChan <- 1
	fmt.Println("yay")
}
