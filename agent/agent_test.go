package agent

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	resourced_config "github.com/resourced/resourced/config"
	_ "github.com/resourced/resourced/readers/docker"
)

func createAgentForTest(t *testing.T) *Agent {
	os.Setenv("RESOURCED_CONFIG_DIR", os.ExpandEnv("$GOPATH/src/github.com/resourced/resourced/tests/resourced-configs"))

	agent, err := New()
	if err != nil {
		t.Fatalf("Initializing agent should work. Error: %v", err)
	}

	return agent
}

func TestRun(t *testing.T) {
	agent := createAgentForTest(t)

	if len(agent.Configs.Readers) <= 0 {
		t.Fatalf("Agent config readers should exist")
	}

	_, err := agent.Run(agent.Configs.Readers[1])
	if err != nil {
		t.Fatalf("Run should work. Error: %v", err)
	}
}

func TestGetRun(t *testing.T) {
	agent := createAgentForTest(t)

	config := agent.Configs.Readers[1]

	_, err := agent.Run(config)
	if err != nil {
		t.Fatalf("Run should work. Error: %v", err)
	}

	output, err := agent.GetRun(config)
	if err != nil {
		t.Fatalf("GetRun should work. Error: %v", err)
	}
	if string(output) == "" {
		t.Errorf("GetRun should return JSON data. Output: %v", string(output))
	}
}

func TestHttpRouter(t *testing.T) {
	agent := createAgentForTest(t)

	_, err := agent.Run(agent.Configs.Readers[0])
	if err != nil {
		t.Fatalf("Run should work. Error: %v", err)
	}

	router := agent.HttpRouter()

	req, err := http.NewRequest("GET", "/r/cpu/info", nil)
	if err != nil {
		t.Errorf("Failed to create HTTP request. Error: %v", err)
	}

	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if jsonData, err := ioutil.ReadAll(resp.Body); err != nil {
		t.Errorf("Failed to read response body. Error: %v", err)
	} else {
		if strings.Contains(string(jsonData), "Error") {
			t.Errorf("jsonData shouldn't return error: %s, %s", jsonData, req.RemoteAddr)
		} else if !strings.Contains(string(jsonData), `UnixNano`) {
			t.Errorf("jsonData does not contain 'UnixNano' key: %s", jsonData)
		} else if !strings.Contains(string(jsonData), `Command`) && !strings.Contains(string(jsonData), `GoStruct`) {
			t.Errorf("jsonData does not contain 'Command' and 'GoStruct' keys: %s", jsonData)
		} else if !strings.Contains(string(jsonData), `Data`) {
			t.Errorf("jsonData does not contain 'Data' key: %s", jsonData)
		}
	}
}

func TestPathWithPrefix(t *testing.T) {
	agent := createAgentForTest(t)

	config := agent.Configs.Readers[1]

	path := agent.pathWithPrefix(config)
	if !strings.HasPrefix(path, "/r") {
		t.Errorf("Path should have been prefixed with /r. Path: %v", path)
	}
	if strings.HasPrefix(path, "/w") {
		t.Errorf("Path is prefixed incorrectly. Path: %v", path)
	}
}

func TestpathWithKindPrefix(t *testing.T) {
	agent := createAgentForTest(t)

	toBeTested := agent.pathWithKindPrefix("r", "/stuff")
	if toBeTested != "/r/stuff" {
		t.Errorf("Path is prefixed incorrectly. toBeTested: %v", toBeTested)
	}

	toBeTested = agent.pathWithKindPrefix("r", "/r/stuff")
	if toBeTested != "/r/stuff" {
		t.Errorf("Path is prefixed incorrectly. toBeTested: %v", toBeTested)
	}

	toBeTested = agent.pathWithKindPrefix("w", "/stuff")
	if toBeTested != "/w/stuff" {
		t.Errorf("Path is prefixed incorrectly. toBeTested: %v", toBeTested)
	}

	toBeTested = agent.pathWithKindPrefix("w", "/w/stuff")
	if toBeTested != "/w/stuff" {
		t.Errorf("Path is prefixed incorrectly. toBeTested: %v", toBeTested)
	}

	toBeTested = agent.pathWithKindPrefix("x", "/stuff")
	if toBeTested != "/w/stuff" {
		t.Errorf("Path is prefixed incorrectly. toBeTested: %v", toBeTested)
	}

	toBeTested = agent.pathWithKindPrefix("x", "/x/stuff")
	if toBeTested != "/x/stuff" {
		t.Errorf("Path is prefixed incorrectly. toBeTested: %v", toBeTested)
	}
}

func TestInitGoStructReader(t *testing.T) {
	agent := createAgentForTest(t)

	var config resourced_config.Config
	for _, c := range agent.Configs.Readers {
		if c.GoStruct == "DockerContainersMemory" {
			config = c
			break
		}
	}

	reader, err := agent.initGoStructReader(config)
	if err != nil {
		t.Fatalf("Initializing Reader should not fail. Error: %v", err)
	}

	goStructField := reflect.ValueOf(reader).Elem().FieldByName("CgroupBasePath")
	if goStructField.String() != "/sys/fs/cgroup/memory/docker" {
		t.Errorf("reader.CgroupBasePath is not set through the config. CgroupBasePath: %v", goStructField.String())
	}
}

func TestInitGoStructWriter(t *testing.T) {
	agent := createAgentForTest(t)

	var config resourced_config.Config
	for _, c := range agent.Configs.Writers {
		if c.GoStruct == "Http" {
			config = c
			break
		}
	}

	writer, err := agent.initGoStructWriter(config)
	if err != nil {
		t.Fatalf("Initializing Writer should not fail. Error: %v", err)
	}

	for field, value := range map[string]string{
		"Url":     "http://localhost:55655/",
		"Method":  "POST",
		"Headers": "X-Token=abc123,X-Teapot-Count=2"} {

		goStructField := reflect.ValueOf(writer).Elem().FieldByName(field)
		if goStructField.String() != value {
			t.Errorf("writer.%s is not set through the config. Url: %v", field, goStructField.String())
		}
	}
}

func TestCommonData(t *testing.T) {
	agent := createAgentForTest(t)

	var config resourced_config.Config
	for _, c := range agent.Configs.Readers {
		if c.GoStruct == "DockerContainersMemory" {
			config = c
			break
		}
	}

	record := agent.commonData(config)
	if len(record) == 0 {
		t.Error("common data should never be empty")
	}
	for _, key := range []string{"UnixNano", "Path", "Interval"} {
		if _, ok := record[key]; !ok {
			t.Errorf("%v data should never be empty.", key)
		}
	}
}

func TestIsAllowed(t *testing.T) {
	_, network, _ := net.ParseCIDR("127.0.0.1/8")
	allowedNetworks := []*net.IPNet{network}

	agent, err := New()
	if err != nil {
		t.Fatalf("Initializing agent should work. Error: %v", err)
	}
	agent.AllowedNetworks = allowedNetworks

	goodIP := "127.0.0.1"
	badIP := "10.0.0.1"
	brokenIP := "batman"

	if !agent.IsAllowed(goodIP) {
		t.Errorf("'%s' should be allowed", goodIP)
	}

	if agent.IsAllowed(badIP) {
		t.Errorf("'%s' should not be allowed", badIP)
	}

	if agent.IsAllowed(brokenIP) {
		t.Errorf("Invalid IP address '%s' should not be allowed ", brokenIP)
	}
}
