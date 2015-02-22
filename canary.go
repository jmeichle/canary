package canary

import (
	"log"
	"os"
	"fmt"
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
	Sensors []sensor.Sensor
	ReloadChan chan bool
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
			fmt.Println("Received SIGINT. Stopping.")
			os.Exit(0)
		case syscall.SIGHUP:
			fmt.Println("Received SIGHUP.")
			c.ReloadChan<-true
		}
		fmt.Println("DONE WITH SIG HANDLER chan receive")
	}
}

func (c *Canary) reloader() {
	if c.ReloadChan == nil {
		c.ReloadChan = make(chan bool)
	}

	for r := range c.ReloadChan {
		if r {
			for _, sensor := range c.Sensors {
				fmt.Println(" Working on sensor: " + sensor.Target.URL)
				sensor.Stop()
			}

			fmt.Println("Sensors should be stopped. Getting a new manifest")

			// get an updated manifest.
			manifest, err := manifest.GetManifest(c.Config.ManifestURL)
			if err != nil {
				log.Fatal(err)
			}
			c.Manifest = manifest

			fmt.Println("Starting new sensors...")

			// Start new sensors:
			c.startSensors()
		}
	}
}

func (c *Canary) createPublishers() {
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
}

func (c *Canary) startSensors() {
	c.Sensors = []sensor.Sensor{} // reset the slice

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
		c.Sensors = append(c.Sensors, sensor)

		go sensor.Start(interval, c.Manifest.StartDelays[index])
	}
}

func (c *Canary) Run() {
	// spinup publishers
	c.createPublishers()
	// create and start sensors
	c.startSensors()
	// start a go routine for watching config reloads
	go c.reloader()
	// start a go routine for measurement publishing.
	go c.publishMeasurements()
}