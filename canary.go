package canary

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/canaryio/canary/pkg/libratopublisher"
	"github.com/canaryio/canary/pkg/sampler"
	"github.com/canaryio/canary/pkg/manifest"
	"github.com/canaryio/canary/pkg/sensor"
	"github.com/canaryio/canary/pkg/stdoutpublisher"
)

type Canary struct {
	Config   Config
	Manifest manifest.Manifest
	OutputChan chan sensor.Measurement
	Publishers []Publisher
}

// New returns a pointer to a new Publsher.
func New() *Canary {
	return &Canary{OutputChan: make(chan sensor.Measurement)}
}

func (c *Canary) publishMeasurements() {
	// publish each incoming measurement
	for m := range c.OutputChan {
		for _, p := range c.Publishers {
			p.Publish(m)
		}
	}
}

func (c *Canary) SignalHandler() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT)
	signal.Notify(signalChan, syscall.SIGHUP)
	for s := range signalChan {
		switch s {
		case syscall.SIGINT:
			os.Exit(0)
		case syscall.SIGHUP:
			// Wouldn't this be nice?
		}
	}
}

func (c *Canary) Run() {
	// spinup publishers
	for _, publisher := range c.Config.PublisherList {
		switch publisher {
		case "stdout":
			p := stdoutpublisher.New()
			c.Publishers = append(c.Publishers, p)
		case "librato":
			p, err := libratopublisher.NewFromEnv()
			if err != nil {
				log.Fatal(err)
			}
			c.Publishers = append(c.Publishers, p)
		default:
			log.Printf("Unknown publisher: %s", publisher)
		}
	}

	// spinup a sensor for each target
	for index, target := range c.Manifest.Targets {
		// Determine whether to use target.Interval or conf.DefaultSampleInterval
		var interval int;
		// Targets that lack an interval value in JSON will have their value set to zero. in this case,
		// use the DefaultSampleInterval
		if target.Interval == 0 {
			interval = c.Config.DefaultSampleInterval
		} else {
			interval = target.Interval
		}
		sensor := sensor.Sensor{
			Target:  target,
			C:       c.OutputChan,
			Sampler: sampler.New(),
		}
		go sensor.Start(interval, c.Manifest.StartDelays[index])
	}

	go c.publishMeasurements()
}