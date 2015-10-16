// +build docker
package docker

import (
	"runtime"
	"strings"
	"testing"
)

func TestNewDockerContainersNetDevRun(t *testing.T) {
	n := NewDockerContainersNetDev()
	err := n.Run()
	if err != nil {
		t.Errorf("Parsing memory data should always be successful. Error: %v", err)
	}
}

func TestNewDockerContainersNetDevToJson(t *testing.T) {
	n := NewDockerContainersNetDev()
	err := n.Run()
	if err != nil {
		t.Errorf("Parsing memory data should always be successful. Error: %v", err)
	}

	jsonData, err := n.ToJson()
	if err != nil {
		t.Errorf("Marshalling memory data should always be successful. Error: %v", err)
	}

	if runtime.GOOS == "darwin" {
		if !strings.Contains(string(jsonData), "Error") {
			t.Errorf("jsonData should return error on darwin: %s", jsonData)
		}
	}

	if runtime.GOOS == "linux" {
		jsonDataString := string(jsonData)

		if strings.Contains(jsonDataString, "Error") {
			t.Errorf("jsonDataString shouldn't return error: %v", jsonDataString)
		}

		keysToTest := []string{"iface", "rxbytes", "rxpackets", "rxerrs", "rxdrop", "rxfifo", "rxframe",
			"rxcompressed", "rxmulticast", "txbytes", "txpackets", "txerrs", "txdrop", "txfifo", "txcolls", "txcarrier", "txcompressed"}

		for _, key := range keysToTest {
			if !strings.Contains(jsonDataString, key) {
				t.Errorf("jsonDataString does not contain '%v' key. jsonDataString: %v", key, jsonDataString)
			}
		}
	}
}
