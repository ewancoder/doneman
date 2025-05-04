package main

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Container struct {
	Name     string   `yaml:"name"`
	Networks []string `yaml:"networks"`
	Status   string   `yaml:"-"`
}

type Config struct {
	Containers []Container `yaml:"containers"`
}

// INIT_WAIT_TIME_SECONDS
const defaultWaitTimeSeconds = 60

// INTERVAL_TIME_SECONDS
const defaultIntervalSeconds = 60

// INTERVAL_NETWORK_RECONNECT_SECONDS
const defaultNetworkReconnectIntervalSeconds = 10

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func createConfigYml() {
	if fileExists("docker-compose.yml") && fileExists(".env") {
		networkPattern := os.Getenv("NETWORK_PATTERN")
		if networkPattern == "" {
			networkPattern = ".*"
		}
		output, err := ConvertDockerComposeToOutput("docker-compose.yml", ".env", networkPattern)
		if err == nil {
			yamlData, err := yaml.Marshal(output)
			if err == nil {
				err = os.WriteFile("config.yml", yamlData, 0644)
				if err != nil {
					fmt.Print(err)
				}
			} else {
				fmt.Print(err)
			}
		} else {
			fmt.Print(err)
		}
	}
}

func main() {
	configPath := "config.yml"

	createConfigYml()
	data, err := os.ReadFile(configPath)
	for {
		i := 0
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			fmt.Printf("Waiting 10 seconds for the correct file to be created...")
			//os.Exit(1)
			i++
			if i > 6 {
				os.Exit(1)
			}

			time.Sleep(10 * time.Second)

			createConfigYml()
			data, err = os.ReadFile(configPath)
			continue
		}

		break
	}

	waitTimeStr := os.Getenv("INIT_WAIT_TIME_SECONDS")
	waitTime := defaultWaitTimeSeconds
	if waitTimeStr != "" {
		parsedTime, err := strconv.Atoi(waitTimeStr)
		if err != nil {
			fmt.Printf("Invalid INIT_WAIT_TIME_SECONDS value: %v\n", err)
			os.Exit(1)
		} else {
			waitTime = parsedTime
		}
	}

	intervalTimeStr := os.Getenv("INTERVAL_TIME_SECONDS")
	intervalTime := defaultIntervalSeconds
	if intervalTimeStr != "" {
		parsedTime, err := strconv.Atoi(intervalTimeStr)
		if err != nil {
			fmt.Printf("Invalid INTERVAL_TIME_SECONDS value: %v\n", err)
			os.Exit(1)
		} else {
			intervalTime = parsedTime
		}
	}

	networkReconnectIntervalStr := os.Getenv("INTERVAL_NETWORK_RECONNECT_SECONDS")
	networkReconnectInterval := defaultNetworkReconnectIntervalSeconds
	if networkReconnectIntervalStr != "" {
		parsedTime, err := strconv.Atoi(networkReconnectIntervalStr)
		if err != nil {
			fmt.Printf("Invalid INTERVAL_NETWORK_RECONNECT_SECONDS value: %v\n", err)
			os.Exit(1)
		} else {
			networkReconnectInterval = parsedTime
		}
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		fmt.Printf("Error creating Docker client: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	fmt.Printf("Waiting for %d seconds before starting...\n", waitTime)
	time.Sleep(time.Duration(waitTime) * time.Second)

	var wg sync.WaitGroup
	for _, cont := range config.Containers {
		wg.Add(1)
		go func(container Container) {
			defer wg.Done()
			for {
				processContainer(cli, container, networkReconnectInterval)
				fmt.Printf("Waiting for %d seconds before checking %s again...\n", intervalTime, container.Name)
				time.Sleep(time.Duration(intervalTime) * time.Second)
			}
		}(cont)
	}
	wg.Wait()
}

func processContainer(cli *client.Client, cont Container, networkReconnectInterval int) {
	fmt.Printf("Checking container %s, necessary networks: %v\n", cont.Name, cont.Networks)

	containerInfo, err := cli.ContainerInspect(context.Background(), cont.Name)
	if err != nil {
		fmt.Printf("Error inspecting container %s: %v\n", cont.Name, err)
		return
	}
	fmt.Printf("Container %s Status: %s\n", cont.Name, containerInfo.State.Status)

	isConnectedToAllNetworks := true
	for _, network := range cont.Networks {
		if _, exists := containerInfo.NetworkSettings.Networks[network]; !exists {
			isConnectedToAllNetworks = false
			break
		}
	}

	if containerInfo.State.Status != "running" || !isConnectedToAllNetworks {
		fmt.Printf("Container %s is not running or not connected to required networks. Disconnecting from networks and re-starting...\n", cont.Name)

		for _, network := range cont.Networks {
			err := cli.NetworkDisconnect(context.Background(), network, cont.Name, true)
			if err != nil {
				fmt.Printf("Error disconnecting container %s from network %s: %v\n", cont.Name, network, err)
				continue
			}
			fmt.Printf("Disconnected container %s from network %s\n", cont.Name, network)
		}

		if err := cli.ContainerStart(context.Background(), cont.Name, container.StartOptions{}); err != nil {
			fmt.Printf("Error starting container %s: %v\n", cont.Name, err)
			return
		}
		fmt.Printf("Container %s started successfully\n", cont.Name)

		for _, network := range cont.Networks {
			for {
				err := cli.NetworkConnect(context.Background(), network, cont.Name, nil)
				if err == nil {
					fmt.Printf("Connected container %s to network %s\n", cont.Name, network)
					break
				}
				fmt.Printf("Error connecting container %s to network %s: %v. Retrying in %d seconds...\n", cont.Name, network, err, networkReconnectInterval)
				time.Sleep(time.Duration(networkReconnectInterval) * time.Second)
			}
		}
	}
}
