package writers

import (
	"encoding/json"
	"github.com/nytlabs/gojsonexplode"
	"github.com/quipo/statsd"
	"time"
)

var statsdBufferedClients map[string]*statsd.StatsdBuffer

// Statsd is a writer that simply serialize all readers data to Statsd.
type Statsd struct {
	Base
	Address        string
	BufferInterval string
	Prefix         string
}

// NewClient builds and returns statsd.StatsdBuffer struct.
func (sd *Statsd) NewClient() (*statsd.StatsdBuffer, error) {
	if sd.Address == "" {
		sd.Address = "localhost:8125"
	}

	if sd.BufferInterval == "" {
		sd.BufferInterval = "1s"
	}

	if statsdBufferedClients == nil {
		statsdBufferedClients = make(map[string]*statsd.StatsdBuffer)
	}

	if _, ok := statsdBufferedClients[sd.BufferInterval]; !ok {
		statsdclient := statsd.NewStatsdClient(sd.Address, sd.Prefix)
		statsdclient.CreateSocket()

		interval, err := time.ParseDuration(sd.BufferInterval)
		if err != nil {
			return nil, err
		}

		statsdBufferedClients[sd.BufferInterval] = statsd.NewStatsdBuffer(interval, statsdclient)
	}

	return statsdBufferedClients[sd.BufferInterval], nil
}

// ToJson serialize Data field to JSON.
func (sd *Statsd) ToJson() ([]byte, error) {
	rawJson, err := json.Marshal(sd.Data)
	if err != nil {
		return rawJson, err
	}

	return gojsonexplode.Explodejson(rawJson, ".")
}
