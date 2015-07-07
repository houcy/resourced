package executors

import (
	"encoding/json"

	"github.com/resourced/resourced/libprocess"
)

type Shell struct {
	Base
	Data map[string]interface{}
}

// Run shells out external program and store the output on c.Data.
func (s *Shell) Run() error {
	output, err := libprocess.NewCmd(s.Command).CombinedOutput()
	s.Data["Output"] = string(output)

	if err != nil {
		s.Data["ExitStatus"] = 1
	} else {
		s.Data["ExitStatus"] = 0
	}

	return nil
}

// ToJson serialize Data field to JSON.
func (s *Shell) ToJson() ([]byte, error) {
	return json.Marshal(s.Data)
}
