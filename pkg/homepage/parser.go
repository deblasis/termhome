package homepage

import (
	"fmt"
	"os"
	"strings"

	"github.com/deblasis/termhome/pkg/logging"
	"gopkg.in/yaml.v3"
)

// LoadSettings loads the settings configuration from the specified YAML file.
func LoadSettings(filePath string) (*Settings, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// It's okay if settings.yaml doesn't exist, return default settings
		if os.IsNotExist(err) {
			return &Settings{Title: "Termdash Homepage"}, nil // Default title
		}
		return nil, fmt.Errorf("failed to read settings file %s: %w", filePath, err)
	}

	var settings Settings
	err = yaml.Unmarshal(data, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings file %s: %w", filePath, err)
	}

	// Provide default title if missing in the file
	if settings.Title == "" {
		settings.Title = "Termdash Homepage"
	}

	return &settings, nil
}

// LoadServices loads the service configurations from the specified YAML file.
func LoadServices(filePath string) ([]*ServiceGroup, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// If services.yaml doesn't exist, return an empty list
		if os.IsNotExist(err) {
			return []*ServiceGroup{}, nil
		}
		return nil, fmt.Errorf("failed to read services file %s: %w", filePath, err)
	}

	logging.Debug("Loading services from %s", filePath)

	// First try to unmarshal as properly formatted YAML with dash prefixes
	var servicesConfig ServicesConfig
	err = yaml.Unmarshal(data, &servicesConfig)

	// If regular unmarshaling fails, try with a different approach for legacy format
	if err != nil {
		logging.Warn("Standard YAML parsing failed, attempting to parse with mixed format support: %v", err)

		// Use a generic map for initial parsing
		var rawMap map[string]interface{}
		err = yaml.Unmarshal(data, &rawMap)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal services file %s: %w", filePath, err)
		}

		// Convert the raw map to our format
		servicesConfig = make(ServicesConfig)

		for key, value := range rawMap {
			// Try to handle it as a list of services
			if servicesList, ok := value.([]interface{}); ok {
				var services []*Service
				for _, serviceItem := range servicesList {
					// Convert each service to our format
					serviceData, err := yaml.Marshal(serviceItem)
					if err != nil {
						logging.Warn("Failed to marshal service data: %v", err)
						continue
					}

					var service Service
					if err := yaml.Unmarshal(serviceData, &service); err != nil {
						logging.Warn("Failed to unmarshal service: %v", err)
						continue
					}

					services = append(services, &service)
				}
				servicesConfig[key] = services
			}
		}
	}

	// Print details of Docker services for debugging
	for groupName, services := range servicesConfig {
		if strings.Contains(groupName, "Docker") {
			logging.Debug("Found Docker group: %s with %d services", groupName, len(services))
			for i, service := range services {
				logging.Debug("Docker service #%d: name=%s, container=%s, server=%s",
					i, service.Name, service.Container, service.Server)
			}
		}
	}

	// Convert map to slice of ServiceGroup for easier iteration
	var serviceGroups []*ServiceGroup
	for groupName, services := range servicesConfig {
		// Ensure services slice is not nil if group exists but is empty
		if services == nil {
			services = []*Service{}
		}
		group := &ServiceGroup{
			Name:     groupName,
			Services: services,
		}
		serviceGroups = append(serviceGroups, group)
	}

	logging.Debug("Loaded %d service groups", len(serviceGroups))
	return serviceGroups, nil
}

// LoadBookmarks loads the bookmark configurations from the specified YAML file.
func LoadBookmarks(filePath string) ([]*BookmarkGroup, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// If bookmarks.yaml doesn't exist, return an empty list
		if os.IsNotExist(err) {
			return []*BookmarkGroup{}, nil
		}
		return nil, fmt.Errorf("failed to read bookmarks file %s: %w", filePath, err)
	}

	var bookmarksConfig BookmarksConfig
	err = yaml.Unmarshal(data, &bookmarksConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal bookmarks file %s: %w", filePath, err)
	}

	// Convert map to slice of BookmarkGroup for easier iteration
	var bookmarkGroups []*BookmarkGroup
	for groupName, bookmarks := range bookmarksConfig {
		// Ensure bookmarks slice is not nil if group exists but is empty
		if bookmarks == nil {
			bookmarks = []*Bookmark{}
		}
		group := &BookmarkGroup{
			Name:      groupName,
			Bookmarks: bookmarks,
		}
		bookmarkGroups = append(bookmarkGroups, group)
	}

	// TODO: Consider sorting groups alphabetically?

	return bookmarkGroups, nil
}

// LoadDockerConfig loads the Docker configuration from the specified YAML file.
func LoadDockerConfig(filePath string) (*DockerConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// If docker.yaml doesn't exist, return nil without error
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read Docker config file %s: %w", filePath, err)
	}

	var dockerConfig map[string]*DockerConfig
	err = yaml.Unmarshal(data, &dockerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal Docker config file %s: %w", filePath, err)
	}

	// For now we'll just use the first Docker configuration found
	// In a future enhancement we could handle multiple Docker servers
	for _, config := range dockerConfig {
		if config != nil {
			// Set default interval if not specified
			if config.Interval <= 0 {
				config.Interval = 60 // Default to 60 seconds
			}
			return config, nil
		}
	}

	// Return a default config if the file exists but is empty
	return &DockerConfig{
		Interval: 60,
		Includes: []string{"*"},
	}, nil
}
