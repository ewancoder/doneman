package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
)

type Config struct {
	Containers []Container `yaml:"containers"`
}

type Container struct {
	Name     string   `yaml:"name"`
	Networks []string `yaml:"networks"`
}

type ToolConfig struct {
	NetworkPattern                  string
	InitWaitTimeSeconds             int
	CheckFrequencySeconds           int
	NetworkReconnectIntervalSeconds int
}

// Loads tool configuration from environment variables.
func LoadToolConfig() (ToolConfig, error) {
	networkPattern := os.Getenv("NETWORK_PATTERN")
	if networkPattern == "" {
		networkPattern = ".*"
	}

	initWaitTimeSecondsStr := os.Getenv("INIT_WAIT_TIME_SECONDS")
	initWaitTimeSeconds := 60
	if initWaitTimeSecondsStr != "" {
		parsedTime, err := strconv.Atoi(initWaitTimeSecondsStr)
		if err != nil {
			fmt.Printf("Invalid INIT_WAIT_TIME_SECONDS value: %v\n", err)
			return ToolConfig{}, err
		} else {
			initWaitTimeSeconds = parsedTime
		}
	}

	checkFrequencySecondsStr := os.Getenv("CHECK_FREQUENCY_SECONDS")
	checkFrequencySeconds := 60
	if checkFrequencySecondsStr != "" {
		parsedTime, err := strconv.Atoi(checkFrequencySecondsStr)
		if err != nil {
			fmt.Printf("Invalid CHECK_FREQUENCY_SECONDS value: %v\n", err)
			return ToolConfig{}, err
		} else {
			checkFrequencySeconds = parsedTime
		}
	}

	networkReconnectIntervalStr := os.Getenv("NETWORK_RECONNECT_INTERVAL_SECONDS")
	networkReconnectInterval := 10
	if networkReconnectIntervalStr != "" {
		parsedTime, err := strconv.Atoi(networkReconnectIntervalStr)
		if err != nil {
			fmt.Printf("Invalid NETWORK_RECONNECT_INTERVAL_SECONDS value: %v\n", err)
			return ToolConfig{}, err
		} else {
			networkReconnectInterval = parsedTime
		}
	}

	return ToolConfig{
		NetworkPattern:                  networkPattern,
		InitWaitTimeSeconds:             initWaitTimeSeconds,
		CheckFrequencySeconds:           checkFrequencySeconds,
		NetworkReconnectIntervalSeconds: networkReconnectInterval,
	}, nil
}

// Tries to create config.yml from docker-compose.yml.
func tryCreateConfigYml(networkPattern string) {
	if _, err := os.Stat("docker-compose.yml"); err == nil {
		output, err := ConvertDockerComposeToConfig("docker-compose.yml", ".env", networkPattern)
		if err != nil {
			fmt.Printf("Error converting docker-compose.yml to config.yml: %v\n", err)
			return
		}

		yamlData, err := yaml.Marshal(output)
		if err != nil {
			fmt.Printf("Error marshalling config.yml: %v\n", err)
			return
		}

		err = os.WriteFile("config.yml", yamlData, 0644)
		if err != nil {
			fmt.Print("Error writing config.yml: %v\n", err)
		}
	}
}
