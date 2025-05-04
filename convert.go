package main

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
)

// Output struct to match the desired format
type Output struct {
	Containers []Container `yaml:"containers"`
}

// removeEnvFileEntries removes env_file entries from compose content
func removeEnvFileEntries(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
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

	return []byte(strings.Join(result, "\n"))
}

// ConvertDockerComposeToOutput converts a Docker Compose file to the specified format
func ConvertDockerComposeToOutput(composeFilePath, envFilePath string, networkPattern string) (Output, error) {
	var output Output

	// Read .env file if provided
	envMap := make(map[string]string)
	if envFilePath != "" {
		envContent, err := ioutil.ReadFile(envFilePath)
		if err != nil {
			return output, fmt.Errorf("failed to read .env file: %v", err)
		}
		for _, line := range strings.Split(string(envContent), "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					envMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// Read Docker Compose file
	composeContent, err := ioutil.ReadFile(composeFilePath)
	if err != nil {
		return output, fmt.Errorf("failed to read compose file: %v", err)
	}

	// Remove env_file entries
	composeContent = removeEnvFileEntries(composeContent)

	// Parse Docker Compose file
	config, err := loader.Load(types.ConfigDetails{
		WorkingDir:  "",
		ConfigFiles: []types.ConfigFile{{Content: composeContent}},
		Environment: envMap,
	})
	if err != nil {
		return output, fmt.Errorf("failed to parse compose file: %v", err)
	}

	// Process each service
	for _, service := range config.Services {
		// Handle replicas
		replicas := 1
		if service.Deploy != nil && service.Deploy.Replicas != nil {
			replicas = int(*service.Deploy.Replicas)
		}

		for i := 1; i <= replicas; i++ {
			projectName := config.Name
			containerName := service.ContainerName
			if containerName == "" {
				if projectName != "" {
					containerName = fmt.Sprintf("%s-%s", projectName, service.Name)
				} else {
					containerName = service.Name
				}
			}
			if replicas > 1 {
				containerName = fmt.Sprintf("%s-%d", containerName, i)
			}

			// Collect networks
			var networks []string
			for networkAlias, _ := range service.Networks {
				if config.Networks != nil {
					if network, exists := config.Networks[networkAlias]; exists {
						networkName := network.Name
						if networkName == "" {
							networkName = networkAlias
						}
						if networkPattern == "" || regexp.MustCompile(networkPattern).MatchString(networkName) {
							networks = append(networks, networkName)
						}
					} else if networkPattern == "" || regexp.MustCompile(networkPattern).MatchString(networkAlias) {
						networks = append(networks, networkAlias)
					}
				} else if networkPattern == "" || regexp.MustCompile(networkPattern).MatchString(networkAlias) {
					networks = append(networks, networkAlias)
				}
			}

			// Add container to output only if it has networks
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
