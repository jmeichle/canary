package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"strconv"

	"github.com/canaryio/canary"
	"github.com/canaryio/canary/pkg/libratopublisher"
	"github.com/canaryio/canary/pkg/sampler"
	"github.com/canaryio/canary/pkg/sensor"
	"github.com/canaryio/canary/pkg/stdoutpublisher"
)

type config struct {
	ManifestURL   string
	DefaultSampleInterval int
	PublisherList []string
}

// builds the app configuration via ENV
func getConfig() (c config, err error) {
	c.ManifestURL = os.Getenv("MANIFEST_URL")
	if c.ManifestURL == "" {
		err = fmt.Errorf("MANIFEST_URL not defined in ENV")
	}

	list := os.Getenv("PUBLISHERS")
	if list == "" {
		list = "stdout"
	}
	c.PublisherList = strings.Split(list, ",")

	interval := os.Getenv("DEFAULT_SAMPLE_INTERVAL")
	// if the variable is unset, an empty string will be returned
	if interval == "" {
		interval = "1"
	}
	c.DefaultSampleInterval, err = strconv.Atoi(interval)
	if err != nil {
		err = fmt.Errorf("DEFAULT_SAMPLE_INTERVAL is not a valid integer")
	}

	return
}

// This will not work for urls on unique intervals. we need a way to average it all out with maths
func even_interval_split(intervalSeconds int, numTargets int) []float64 {
	var intervalMilliseconds = float64(intervalSeconds*1000)
	var arr = make([]float64, numTargets) // Create an float64 slice of numTargets long.
									      // This will be an array of start delay times that evenly add up to interval (ms)
	var chunkSize = float64(intervalMilliseconds/float64(numTargets))
	for i := 0.0; i < intervalMilliseconds; i = i + chunkSize {
		arr[int((i/chunkSize))] = i
	}
	return arr
}

func main() {
	conf, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}

	manifest, err := canary.GetManifest(conf.ManifestURL)
	if err != nil {
		log.Fatal(err)
	}

	// output chan
	c := make(chan sensor.Measurement)

	var publishers []canary.Publisher

	// spinup publishers
	for _, publisher := range conf.PublisherList {
		switch publisher {
		case "stdout":
			p := stdoutpublisher.New()
			publishers = append(publishers, p)
		case "librato":
			p, err := libratopublisher.NewFromEnv()
			if err != nil {
				log.Fatal(err)
			}
			publishers = append(publishers, p)
		default:
			log.Printf("Unknown publisher: %s", publisher)
		}
	}

	var delaySlice = even_interval_split(conf.DefaultSampleInterval, len(manifest.Targets))

	// spinup a sensor for each target
	for index, target := range manifest.Targets {
		// Determine whether to use target.Interval or conf.DefaultSampleInterval
		var interval int;
		// Targets that lack an interval value in JSON will have their value set to zero. in this case,
		// use the DefaultSampleInterval
		if target.Interval == 0 {
			interval = conf.DefaultSampleInterval
		} else {
			interval = target.Interval
		}
		sensor := sensor.Sensor{
			Target:  target,
			C:       c,
			Sampler: sampler.New(),
		}
		go sensor.Start(interval, delaySlice[index])
	}

	// publish each incoming measurement
	for m := range c {
		for _, p := range publishers {
			p.Publish(m)
		}
	}
}
