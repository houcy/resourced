package writers

import (
	"encoding/json"
)

// NewStatsdAbsolute is StatsdAbsolute constructor.
func NewStatsdAbsolute() *StatsdAbsolute {
	sd := &StatsdAbsolute{}
	return sd
}

// StatsdAbsolute is a writer that simply serialize all readers data to StatsdAbsolute.
type StatsdAbsolute struct {
	Statsd
}

// Run executes the writer.
func (sd *StatsdAbsolute) Run() error {
	sd.Data = sd.GetReadersData()
	dataJson, err := sd.ToJson()

	if err != nil {
		return err
	}

	var data map[string]interface{}

	err = json.Unmarshal(dataJson, &data)
	if err != nil {
		return err
	}

	client, err := sd.NewClient()
	if err != nil {
		return err
	}

	for key, value := range data {
		if valueFloat, ok := value.(float64); ok {
			client.FAbsolute(key, valueFloat)
		}
	}

	return nil
}
