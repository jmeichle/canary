package canary

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/canaryio/canary/pkg/sampler"
)

// Manifest represents configuration data.
type Manifest struct {
	Targets []sampler.Target
	StartDelays []float64
}

// GenerateRampupDelays accepts the time range to generate delays for, and the number of targets
// as integers. It returns a float array of numTargets size
func (m *Manifest) GenerateRampupDelays(intervalSeconds int) {
	var intervalMilliseconds = float64(intervalSeconds*1000)
	var arr = make([]float64, len(m.Targets))
	var chunkSize = float64(intervalMilliseconds/float64(len(m.Targets)))
	for i := 0.0; i < intervalMilliseconds; i = i + chunkSize {
		arr[int((i/chunkSize))] = i
	}
	m.StartDelays = arr
}

// GetManifest retreives a manifest from a given URL.
func GetManifest(url string) (manifest Manifest, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, &manifest)
	if err != nil {
		return
	}

	// Initialize manifest.StartDelays to zeros
	manifest.StartDelays = make([]float64, len(manifest.Targets))
	for i := 0; i < len(manifest.Targets); i++ {
		manifest.StartDelays[i] = 0.0
	}

	return
}
