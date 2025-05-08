package main

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func updateHealthCheckFile() {
	filePath := os.Getenv("HEALTHCHECK_FILE")
	if filePath == "" {
		fmt.Print("HEALTHCHECK_FILE environment variable is not set.")
		return
	}

	for {
		currentTime := time.Now().String()
		err := os.WriteFile(filePath, []byte(currentTime), 0644)
		if err != nil {
			fmt.Printf("Failed to write to health check file: %v\n", err)
		}
		time.Sleep(30 * time.Second)
	}
}

func main() {
	cfg, err := LoadToolConfig()
	if err != nil {
		fmt.Printf("Error loading tool config: %v\n", err)
		os.Exit(1)
	}

	tryCreateConfigYml(cfg.NetworkPattern)
	data, err := os.ReadFile("config.yml")
	for {
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			fmt.Printf("Waiting 10 seconds for the correct file to be created...\n")

			time.Sleep(10 * time.Second)

			tryCreateConfigYml(cfg.NetworkPattern)
			data, err = os.ReadFile("config.yml")
			continue
		}

		break
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Printf("Error creating Docker client: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	fmt.Printf("Waiting for %d seconds before starting...\n", cfg.InitWaitTimeSeconds)
	time.Sleep(time.Duration(cfg.InitWaitTimeSeconds) * time.Second)

	var wg sync.WaitGroup
	for _, cont := range config.Containers {
		wg.Add(1)
		go func(container Container) {
			defer wg.Done()
			for {
				processContainer(cli, container, cfg.NetworkReconnectIntervalSeconds)
				fmt.Printf("Waiting for %d seconds before checking %s again...\n", cfg.CheckFrequencySeconds, container.Name)
				time.Sleep(time.Duration(cfg.CheckFrequencySeconds) * time.Second)
			}
		}(cont)
	}

	go updateHealthCheckFile()
	wg.Wait()
}

// Checks a single container, restarts & reconnects if needed.
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

	if containerInfo.State.Status == "running" && isConnectedToAllNetworks {
		return
	}

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
			containerInfo, err := cli.ContainerInspect(context.Background(), cont.Name)
			if containerInfo.State.Status != "running" {
				// Avoid dead locking the process.
				fmt.Printf("Container was stopped, skipping network reconnection.")
				break
			}

			err = cli.NetworkConnect(context.Background(), network, cont.Name, nil)
			if err == nil {
				fmt.Printf("Connected container %s to network %s\n", cont.Name, network)
				break
			}

			fmt.Printf("Error connecting container %s to network %s: %v. Retrying in %d seconds...\n", cont.Name, network, err, networkReconnectInterval)
			time.Sleep(time.Duration(networkReconnectInterval) * time.Second)
		}
	}
}
