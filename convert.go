package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"github.com/joho/godotenv"
)

// Converts docker-compose.yml to config.yml.
func ConvertDockerComposeToConfig(composeFileName, envFileName, networkPattern string) (Config, error) {
	var output Config
	envMap := make(map[string]string)

	// Read .env file.
	if _, err := os.Stat(envFileName); err == nil {
		readEnvMap, err := godotenv.Read(envFileName)
		if err != nil {
			fmt.Errorf("Failed to read .env file, proceeding with empty env: %v", err)
		} else {
			envMap = readEnvMap
		}
	}

	// Read docker-compose.yml file.
	composeContent, err := os.ReadFile(composeFileName)
	if err != nil {
		return output, fmt.Errorf("Failed to read compose file: %v", err)
	}

	// Remove env_file entries from docker-compose.yml file.
	composeStringContent := removeEnvFileEntries(string(composeContent))
	composeContent = []byte(composeStringContent)

	// Parse Docker Compose file.
	config, err := loader.LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Content: composeContent}},
		Environment: envMap,
	})
	if err != nil {
		return output, fmt.Errorf("Failed to parse compose file: %v", err)
	}

	projectName := config.Name
	if projectName == "" {
		return output, fmt.Errorf("Failed to get project name from compose file")
	}

	if config.Networks == nil {
		return output, fmt.Errorf("Failed to get networks from compose file.")
	}

	// Process each service.
	for _, service := range config.Services {
		replicas := 1
		if service.Deploy != nil && service.Deploy.Replicas != nil {
			replicas = int(*service.Deploy.Replicas)
		}

		for i := 1; i <= replicas; i++ {
			containerName := service.ContainerName
			if containerName == "" {
				containerName = fmt.Sprintf("%s-%s", projectName, service.Name)
				if replicas > 1 {
					containerName = fmt.Sprintf("%s-%d", containerName, i)
				}
			}
			var networks []string
			for networkAlias, _ := range service.Networks {
				if network, exists := config.Networks[networkAlias]; exists {
					networkName := network.Name
					if networkName == "" {
						return output, fmt.Errorf("Failed to get network name from compose file.")
					}
					if networkPattern == "" || regexp.MustCompile(networkPattern).MatchString(networkName) {
						networks = append(networks, networkName)
					}
				} else {
					return output, fmt.Errorf("Failed to get network alias from compose file.")
				}
			}

			// Add containers only when they have networks.
			if len(networks) > 0 {
				output.Containers = append(output.Containers, Container{
					Name:     containerName,
					Networks: networks,
				})
			}
		}
	}

	return output, nil
}

// Removes env_file entries from compose content.
func removeEnvFileEntries(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	skip := false

	for _, line := range lines {
		if strings.Contains(line, "env_file:") {
			skip = true
			continue
		}
		if skip && strings.HasPrefix(strings.TrimSpace(line), "-") {
			continue
		}
		skip = false
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
