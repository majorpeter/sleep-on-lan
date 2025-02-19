package main

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/juju/loggo"
)

const (
	COMMAND_TYPE_EXTERNAL     = "external"
	COMMAND_TYPE_INTERNAL_DLL = "internal-dll"
)

type Configuration struct {
	Listeners                  []string // what is read from the sol.json configuration file
	LogLevel                   string
	BroadcastIP                string
	ExitIfAnyPortIsAlreadyUsed bool
	Commands                   []CommandConfiguration // the various defined commands. Will be enhanded with default operation if empty from configuration
	Auth                       AuthConfiguration      // optional
	HTTPOutput                 string
	AvoidDualUDPSending        AvoidDualUDPSendingConfiguration

	listenersConfiguration []ListenerConfiguration // converted once parsed from Listeners (= internal representation, not an external configuration)
}

type AvoidDualUDPSendingConfiguration struct {
	AvoidDualUDPSendingActive bool   `json:"Active"`
	AvoidDualUDPSendingDelay  string `json:"Delay"`
}

type AuthConfiguration struct {
	Login    string `json:"Login"`
	Password string `json:"Password"`
}

func (a AuthConfiguration) isEmpty() bool {
	return a.Login == "" && a.Password == ""
}

type CommandConfiguration struct {
	Operation   string `json:"Operation"`
	Command     string `json:"Command"`
	IsDefault   bool   `json:"Default"`
	CommandType string `json:"Type"`
}

type ListenerConfiguration struct {
	active bool
	port   int
	nature string
}

func (conf *Configuration) InitDefaultConfiguration() {
	conf.Listeners = []string{"UDP:9", "HTTP:8009"}
	conf.LogLevel = "INFO"
	conf.BroadcastIP = "192.168.255.255"
	conf.HTTPOutput = "XML"
	conf.ExitIfAnyPortIsAlreadyUsed = false
	conf.AvoidDualUDPSending = AvoidDualUDPSendingConfiguration{AvoidDualUDPSendingActive: false, AvoidDualUDPSendingDelay: "100ms"}
	// default commands are registered on Parse() method, depending on the current operating system
}

func (conf *Configuration) Load(configurationFileName string) error {
	if _, err := os.Stat(configurationFileName); err == nil {
		logger.Infof("Configuration file found under [" + colorer.Green(configurationFileName) + "], now reading content")
		file, _ := os.Open(configurationFileName)
		decoder := json.NewDecoder(file)
		decoder.DisallowUnknownFields()
		err := decoder.Decode(&conf)
		if err != nil {
			return err
		}
	} else {
		logger.Infof("No external configuration file found under [" + colorer.Red(configurationFileName) + "], will use default values")
	}

	return nil
}

func (conf *Configuration) RefineLogger() error {
	// Gestion logs
	switch conf.LogLevel {
	case "NONE", "OFF":
		logger.SetLogLevel(loggo.CRITICAL)
	case "DEBUG":
		logger.SetLogLevel(loggo.DEBUG)
	case "INFO":
		logger.SetLogLevel(loggo.INFO)
	case "WARN", "WARNING":
		logger.SetLogLevel(loggo.WARNING)
	case "ERROR":
		logger.SetLogLevel(loggo.ERROR)
	default:
		logger.Errorf("unrecognized log level[" + colorer.Red(conf.LogLevel) + "], allowed are NONE or OFF, DEBUG, INFO, WARN or WARNING, ERROR")
		return errors.New("unknown log level")
	}
	return nil
}

func (conf *Configuration) Parse() error {
	// Convert activated ports
	for _, s := range conf.Listeners {
		var splitted = strings.Split(s, ":")
		var key = splitted[0]
		var listenerConfiguration = new(ListenerConfiguration)
		listenerConfiguration.active = true
		if len(splitted) == 2 {
			listenerConfiguration.port, _ = strconv.Atoi(splitted[1])
		}
		if strings.EqualFold(key, "UDP") {
			listenerConfiguration.nature = "UDP"
			conf.listenersConfiguration = append(conf.listenersConfiguration, *listenerConfiguration)
		} else if strings.EqualFold(key, "HTTP") {
			listenerConfiguration.nature = "HTTP"
			conf.listenersConfiguration = append(conf.listenersConfiguration, *listenerConfiguration)
		} else {
			logger.Errorf("Unknown listener type [" + key + "], valid values are : UDP, HTTP")
		}
	}
	logger.Debugf("Configuration loaded", conf)

	// If only one command, then force default, and if no commands are found, inject default ones
	var nbCommands = len(conf.Commands)
	if nbCommands == 0 {
		RegisterDefaultCommand()
	} else if nbCommands == 1 {
		if !conf.Commands[0].IsDefault {
			logger.Warningf("Only one command found in configuration, [" + colorer.Green(conf.Commands[0].Operation) + "], and this command is not set as default : forcing default")
			conf.Commands[0].IsDefault = true
		}
	}

	// Set type to external if not provided
	for idx := range conf.Commands {
		command := &conf.Commands[idx]
		if command.CommandType == "" {
			logger.Warningf("Forcing type to [EXTERNAL] for command [" + command.Operation + "]")
			command.CommandType = COMMAND_TYPE_EXTERNAL
		}
	}

	// Reject commands with improper types
	improperCommandTypesFound := false
	for idx := range conf.Commands {
		command := &conf.Commands[idx]
		if command.CommandType != COMMAND_TYPE_EXTERNAL && command.CommandType != COMMAND_TYPE_INTERNAL_DLL {
			logger.Errorf("Command [" + colorer.Green(command.Operation) + "] is configured with an improper type [" + colorer.Red(command.CommandType) + "] (only '" + colorer.Green(COMMAND_TYPE_INTERNAL_DLL) + "' or '" + colorer.Green(COMMAND_TYPE_EXTERNAL) + "' are allowed)")
			improperCommandTypesFound = true
		}
	}
	if improperCommandTypesFound {
		return errors.New("some improper command types defined")
	}

	// Stop policy
	if conf.ExitIfAnyPortIsAlreadyUsed {
		logger.Infof("Daemon will stop if any listener can't be started (per `ExitIfAnyPortIsAlreadyUsed` configuration)")
	} else {
		logger.Infof("Daemon won't stop even if one listener can't be started (per `ExitIfAnyPortIsAlreadyUsed` configuration)")
	}

	// Avoid dual UDP sending
	if conf.AvoidDualUDPSending.AvoidDualUDPSendingActive {
		logger.Infof("Avoid dual UDP sending enabled, delay is [" + conf.AvoidDualUDPSending.AvoidDualUDPSendingDelay + "]")
	} else {
		logger.Infof("Avoid dual UDP sending not enabled")
	}

	return nil
}
