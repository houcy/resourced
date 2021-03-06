// Package agent runs readers, writers, and HTTP server.
package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	gocache "github.com/patrickmn/go-cache"
	"github.com/satori/go.uuid"

	resourced_config "github.com/resourced/resourced/config"
	"github.com/resourced/resourced/executors"
	"github.com/resourced/resourced/host"
	"github.com/resourced/resourced/libmap"
	"github.com/resourced/resourced/libtime"
	"github.com/resourced/resourced/loggers"
	"github.com/resourced/resourced/readers"
	"github.com/resourced/resourced/writers"
)

// New is the constructor for Agent struct.
func New() (*Agent, error) {
	agent := &Agent{}

	agent.ID = uuid.NewV4().String()

	err := agent.setConfigs()
	if err != nil {
		return nil, err
	}

	err = agent.setTags()
	if err != nil {
		return nil, err
	}

	err = agent.setAccessTokens()
	if err != nil {
		return nil, err
	}

	agent.ResultDB = gocache.New(time.Duration(agent.GeneralConfig.TTL)*time.Second, 10*time.Second)
	agent.GraphiteDB = libmap.NewTSafeNestedMapInterface(nil)
	agent.ExecutorCounterDB = libmap.NewTSafeMapCounter(nil)
	agent.TCPLogDB = libmap.NewTSafeMapStrings(map[string][]string{
		"Loglines": make([]string, 0),
	})

	return agent, err
}

// Agent struct carries most of the functionality of ResourceD.
// It collects information through readers and serve them up as HTTP+JSON.
type Agent struct {
	ID                string
	Tags              map[string]string
	AccessTokens      []string
	Configs           *resourced_config.Configs
	GeneralConfig     resourced_config.GeneralConfig
	DbPath            string
	ResultDB          *gocache.Cache
	GraphiteDB        *libmap.TSafeNestedMapInterface
	ExecutorCounterDB *libmap.TSafeMapCounter
	TCPLogDB          *libmap.TSafeMapStrings
}

// Run executes a reader/writer/executor/log config.
func (a *Agent) Run(config resourced_config.Config) (output []byte, err error) {
	if config.GoStruct != "" && config.Kind == "reader" {
		output, err = a.runGoStructReader(config)
	} else if config.GoStruct != "" && config.Kind == "writer" {
		output, err = a.runGoStructWriter(config)
	} else if config.GoStruct != "" && config.Kind == "executor" {
		output, err = a.runGoStructExecutor(config)
	}

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Error":              err.Error(),
			"config.GoStruct":    config.GoStruct,
			"config.Path":        config.Path,
			"config.Interval":    config.Interval,
			"config.Kind":        config.Kind,
			"config.ReaderPaths": fmt.Sprintf("%s", config.ReaderPaths),
		}).Error("Failed to execute runGoStructReader/runGoStructWriter/runGoStructExecutor")
	}

	err = a.saveRun(config, output, err)

	return output, err
}

// initGoStructReader initialize and return IReader.
func (a *Agent) initGoStructReader(config resourced_config.Config) (readers.IReader, error) {
	return readers.NewGoStructByConfig(config)
}

// initGoStructWriter initialize and return IWriter.
func (a *Agent) initGoStructWriter(config resourced_config.Config) (writers.IWriter, error) {
	writer, err := writers.NewGoStructByConfig(config)
	if err != nil {
		return nil, err
	}

	// Set configs data.
	writer.SetConfigs(a.Configs)

	// Get readers data.
	readersData := make(map[string][]byte)

	for _, readerPath := range config.ReaderPaths {
		if strings.HasSuffix(readerPath, "/graphite") {
			// Special Case: if readerPath contains /graphite
			record := a.commonGraphiteData()
			record["Data"] = a.GraphiteDB.All()

			readerJsonBytes, err := json.Marshal(record)
			if err == nil {
				readersData[readerPath] = readerJsonBytes
			}

		} else {
			// Normal Case: regular /r/reader
			readerJsonBytes, err := a.GetRunByPath(config.PathWithKindPrefix("r", readerPath))
			if err == nil {
				readersData[readerPath] = readerJsonBytes
			}
		}
	}

	writer.SetReadersDataInBytes(readersData)

	return writer, err
}

// initResourcedMasterWriter initialize ResourceD Master specific IWriter.
func (a *Agent) initResourcedMasterWriter(config resourced_config.Config) (writers.IWriter, error) {
	var apiPath string

	if config.GoStruct == "ResourcedMasterHost" {
		apiPath = "/api/hosts"
	}

	urlFromConfigInterface, ok := config.GoStructFields["Url"]
	if !ok || urlFromConfigInterface == nil { // Check if Url is not defined in config
		config.GoStructFields["Url"] = a.GeneralConfig.ResourcedMaster.URL + apiPath

	} else { // Check if Url does not contain apiPath
		urlFromConfig := urlFromConfigInterface.(string)
		if !strings.HasSuffix(urlFromConfig, apiPath) {
			config.GoStructFields["Url"] = a.GeneralConfig.ResourcedMaster.URL + apiPath
		}
	}

	// Check if username is not defined
	// If so, set GeneralConfig.ResourcedMaster.AccessToken as default
	usernameFromConfigInterface, ok := config.GoStructFields["Username"]
	if !ok || usernameFromConfigInterface == nil {
		config.GoStructFields["Username"] = a.GeneralConfig.ResourcedMaster.AccessToken

	}

	return a.initGoStructWriter(config)
}

// initGoStructExecutor initialize and return IExecutor.
func (a *Agent) initGoStructExecutor(config resourced_config.Config) (executors.IExecutor, error) {
	executor, err := executors.NewGoStructByConfig(config)
	if err != nil {
		return nil, err
	}

	goodItems := libmap.AllNonExpiredCache(a.ResultDB)
	goodItemsInBytes := make(map[string][]byte)

	for key, item := range goodItems {
		goodItemsInBytes[key] = item.Object.([]byte)
	}

	executor.SetReadersDataInBytes(goodItemsInBytes)
	executor.SetCounterDB(a.ExecutorCounterDB)
	executor.SetTags(a.Tags)

	// Check if ResourcedMasterURL is not defined
	// If so, set GeneralConfig.ResourcedMaster.URL as default
	if config.ResourcedMasterURL == "" {
		executor.SetResourcedMasterURL(a.GeneralConfig.ResourcedMaster.URL)
	}

	// Check if ResourcedMasterAccessToken is not defined
	// If so, set GeneralConfig.ResourcedMaster.AccessToken as default
	if config.ResourcedMasterAccessToken == "" {
		executor.SetResourcedMasterAccessToken(a.GeneralConfig.ResourcedMaster.AccessToken)
	}

	host, err := a.hostData()
	if err != nil {
		return nil, err
	}

	executor.SetHostData(host)

	return executor, nil
}

// runGoStruct executes Run() fom IReader/IWriter/IExecutor and returns the output.
// Note that IWriter and IExecutor also implement IReader.
func (a *Agent) runGoStruct(readerOrWriterOrExecutor readers.IReader) ([]byte, error) {
	err := readerOrWriterOrExecutor.Run()
	if err != nil {
		errData := make(map[string]string)
		errData["Error"] = err.Error()
		return json.Marshal(errData)
	}

	return readerOrWriterOrExecutor.ToJson()
}

// runGoStructReader executes IReader and returns the output.
func (a *Agent) runGoStructReader(config resourced_config.Config) ([]byte, error) {
	// Initialize IReader
	reader, err := a.initGoStructReader(config)
	if err != nil {
		return nil, err
	}

	return a.runGoStruct(reader)
}

// runGoStructWriter executes IWriter and returns error if exists.
func (a *Agent) runGoStructWriter(config resourced_config.Config) ([]byte, error) {
	var writer writers.IWriter
	var err error

	// Initialize IWriter
	if strings.HasPrefix(config.GoStruct, "ResourcedMaster") {
		writer, err = a.initResourcedMasterWriter(config)
		if err != nil {
			return nil, err
		}

	} else {
		writer, err = a.initGoStructWriter(config)
		if err != nil {
			return nil, err
		}
	}

	err = writer.GenerateData()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Error":              err.Error(),
			"config.GoStruct":    config.GoStruct,
			"config.Path":        config.Path,
			"config.Interval":    config.Interval,
			"config.Kind":        config.Kind,
			"config.ReaderPaths": fmt.Sprintf("%s", config.ReaderPaths),
		}).Error("Failed to execute writer.GenerateData()")

		return nil, err
	}

	return a.runGoStruct(writer)
}

// runGoStructExecutor executes IExecutor and returns the output.
func (a *Agent) runGoStructExecutor(config resourced_config.Config) ([]byte, error) {
	var executor executors.IExecutor
	var err error

	// Initialize IExecutor
	executor, err = a.initGoStructExecutor(config)
	if err != nil {
		return nil, err
	}

	return a.runGoStruct(executor)
}

// commonGraphiteData gathers common information for graphite reader.
func (a *Agent) commonGraphiteData() map[string]interface{} {
	record := make(map[string]interface{})
	record["UnixNano"] = time.Now().UnixNano()
	record["Path"] = "/graphite"

	host, err := a.hostData()
	if err == nil {
		record["Host"] = host
	}

	return record
}

// hostData builds host related information.
func (a *Agent) hostData() (*host.Host, error) {
	h, err := host.NewHostByHostname()
	if err != nil {
		return nil, err
	}

	h.Tags = a.Tags

	return h, nil
}

// saveRun gathers basic, host, and reader/witer information and save them into local storage.
func (a *Agent) saveRun(config resourced_config.Config, output []byte, err error) error {
	// Do not perform save if config.Path is empty.
	if config.Path == "" {
		return nil
	}

	// Do not perform save if both output and error are empty.
	if output == nil && err == nil {
		return nil
	}

	record := config.CommonJsonData()

	host, err := a.hostData()
	if err != nil {
		return err
	}
	record["Host"] = host

	if err == nil {
		runData := new(interface{})
		err = json.Unmarshal(output, &runData)
		if err != nil {
			return err
		}
		record["Data"] = runData

	} else {
		errMap := make(map[string]string)
		errMap["Error"] = err.Error()
		record["Data"] = errMap
	}

	recordInJson, err := json.Marshal(record)
	if err != nil {
		return err
	}

	a.ResultDB.Set(config.PathWithPrefix(), recordInJson, gocache.DefaultExpiration)

	return err
}

// GetRunByPath returns JSON data stored in local storage given path string.
func (a *Agent) GetRunByPath(path string) ([]byte, error) {
	valueInterface, found := a.ResultDB.Get(path)
	if found {
		return valueInterface.([]byte), nil
	}
	return nil, nil
}

// RunForever executes Run() in an infinite loop with a sleep of config.Interval.
func (a *Agent) RunForever(config resourced_config.Config) {
	go func(config resourced_config.Config) {
		for {
			a.Run(config)
			libtime.SleepString(config.Interval)
		}
	}(config)
}

// RunAllForever runs everything in an infinite loop.
func (a *Agent) RunAllForever() {
	for _, config := range a.Configs.Readers {
		a.RunForever(config)
	}
	for _, config := range a.Configs.Writers {
		a.RunForever(config)
	}
	for _, config := range a.Configs.Executors {
		a.RunForever(config)
	}
	for _, config := range a.Configs.Loggers {
		logger, err := loggers.NewGoStructByConfig(config)
		if err != nil {
			continue
		}

		go func() {
			logger.RunBlocking()
		}()

		go func(config resourced_config.Config, logger loggers.ILogger) {
			for {
				loglines, err := a.SendLog(logger.GetData(), logger.GetFile())
				if err != nil {
					libtime.SleepString(config.Interval)
					continue
				}

				outputJson, err := json.Marshal(loglines)
				if err != nil {
					libtime.SleepString(config.Interval)
					continue
				}

				a.saveRun(config, outputJson, err)
				a.PruneLogs(logger, logger.GetData())
				libtime.SleepString(config.Interval)
			}
		}(config, logger)
	}
	a.SendTCPLogForever(a.GeneralConfig.LogReceiver)
}
