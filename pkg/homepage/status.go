package homepage

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/deblasis/termhome/pkg/logging"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// StatusState represents the state of a service
type StatusState string

const (
	// StatusUnknown indicates the status is unknown or has not been checked
	StatusUnknown StatusState = "unknown"
	// StatusOK indicates the service is up
	StatusOK StatusState = "ok"
	// StatusWarning indicates the service is experiencing issues
	StatusWarning StatusState = "warning"
	// StatusCritical indicates the service is down
	StatusCritical StatusState = "critical"
)

// StatusUpdateFunc is a callback function called when a status changes
type StatusUpdateFunc func(serviceName string, state StatusState, message string)

// StatusResult represents the result of a status check
type StatusResult struct {
	State        StatusState   // The state of the service (OK, Warning, Critical, Unknown)
	Message      string        // A message with additional information (e.g. response time)
	ResponseTime time.Duration // Time it took to get a response
	LastChecked  time.Time     // When the status was last checked
}

// StatusMonitor manages the status checking for services
type StatusMonitor struct {
	services       map[string]*Service      // Map of service names to services
	results        map[string]*StatusResult // Map of service names to status results
	stopChannels   map[string]chan struct{} // Channels to stop the monitoring goroutines
	updateFunc     StatusUpdateFunc         // Function to call when a status changes
	globalInterval int                      // Global interval override from settings
	mutex          sync.RWMutex             // For thread-safe access to results map
}

// NewStatusMonitor creates a new status monitor
func NewStatusMonitor(updateFunc StatusUpdateFunc) *StatusMonitor {
	return &StatusMonitor{
		services:       make(map[string]*Service),
		results:        make(map[string]*StatusResult),
		stopChannels:   make(map[string]chan struct{}),
		updateFunc:     updateFunc,
		globalInterval: 0, // No global override by default
		mutex:          sync.RWMutex{},
	}
}

// AddService adds a service to be monitored
func (sm *StatusMonitor) AddService(service *Service) {
	if service.DisableStatus {
		logging.Info("Status monitoring disabled for service %s", service.Name)
		return
	}

	// Check if Docker container monitoring is enabled
	hasDockerMonitoring := service.Container != ""

	// Don't monitor if no monitoring config is provided
	if service.Ping == nil && service.SiteMonitor == nil && service.Status == "" && !hasDockerMonitoring {
		logging.Debug("Service %s has no monitoring configuration, not adding to monitor", service.Name)
		return
	}

	// Add logging for Docker container service
	if hasDockerMonitoring {
		logging.Debug("Adding Docker container service %s to status monitor (container=%s, server=%s)",
			service.Name, service.Container, service.Server)
	} else {
		logging.Info("Adding service %s to status monitor", service.Name)
	}

	sm.mutex.Lock()
	sm.services[service.Name] = service
	sm.results[service.Name] = &StatusResult{
		State:       StatusUnknown,
		Message:     "",
		LastChecked: time.Time{},
	}
	sm.mutex.Unlock()

	// If there's a static status provided, use it as initial state
	if service.Status != "" {
		state := StatusUnknown
		switch strings.ToLower(service.Status) {
		case "ok":
			state = StatusOK
		case "warning":
			state = StatusWarning
		case "critical":
			state = StatusCritical
		}
		// Use empty message for static status to avoid showing "Initial static status"
		sm.updateServiceStatus(service.Name, state, "")
	} else if hasDockerMonitoring {
		// For Docker container services without a static status, set initial state to unknown
		sm.updateServiceStatus(service.Name, StatusUnknown, "Waiting for container status...")
	}

	// Start the monitoring goroutine for this service
	sm.startMonitoring(service)
}

// AddDockerMonitoring adds Docker container monitoring
func (sm *StatusMonitor) AddDockerMonitoring(config *DockerConfig) error {
	if config == nil {
		return nil
	}

	// Test Docker client connection
	cli, err := createDockerClient(config)
	if err != nil {
		return fmt.Errorf("failed to connect to Docker: %w", err)
	}
	cli.Close()

	interval := config.Interval
	if interval <= 0 {
		interval = 60 // Default to 60 seconds
	}

	stopChan := make(chan struct{})
	sm.stopChannels["docker"] = stopChan

	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		// Do an initial check immediately
		sm.checkDockerContainers(config)

		for {
			select {
			case <-ticker.C:
				sm.checkDockerContainers(config)
			case <-stopChan:
				return
			}
		}
	}()

	return nil
}

// GetStatus returns the current status of a service
func (sm *StatusMonitor) GetStatus(serviceName string) *StatusResult {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result, exists := sm.results[serviceName]
	if !exists {
		return &StatusResult{
			State:       StatusUnknown,
			Message:     "Service not monitored",
			LastChecked: time.Time{},
		}
	}
	return result
}

// Stop stops all monitoring goroutines
func (sm *StatusMonitor) Stop() {
	logging.Info("Stopping status monitor")
	for _, stopChan := range sm.stopChannels {
		close(stopChan)
	}

	// Clear channels
	sm.stopChannels = make(map[string]chan struct{})
}

// startMonitoring starts the monitoring goroutine for a service
func (sm *StatusMonitor) startMonitoring(service *Service) {
	stopChan := make(chan struct{})
	sm.stopChannels[service.Name] = stopChan

	if service.Ping != nil {
		// Set default values if not specified
		count := service.Ping.Count
		if count <= 0 {
			count = 3
		}

		// Use global interval if set, otherwise use service-specific or default
		interval := service.Ping.Interval
		if sm.globalInterval > 0 {
			interval = sm.globalInterval
		} else if interval <= 0 {
			interval = 60 // Default to 60 seconds
		}

		host := service.Ping.Host
		if host == "" {
			host = service.Href
			// Extract host from URL if it's a URL
			if strings.HasPrefix(host, "http") {
				host = strings.TrimPrefix(host, "http://")
				host = strings.TrimPrefix(host, "https://")
				host = strings.Split(host, "/")[0]
				host = strings.Split(host, ":")[0]
			}
		}

		logging.Info("Starting ping monitoring for %s with interval %d seconds", service.Name, interval)

		// Start ping monitoring goroutine
		go func() {
			ticker := time.NewTicker(time.Duration(interval) * time.Second)
			defer ticker.Stop()

			// Do an initial ping immediately
			sm.pingService(service.Name, host, count)

			for {
				select {
				case <-ticker.C:
					sm.pingService(service.Name, host, count)
				case <-stopChan:
					return
				}
			}
		}()
	} else if service.SiteMonitor != nil {
		// Set default values if not specified
		// Use global interval if set, otherwise use service-specific or default
		interval := service.SiteMonitor.Interval
		if sm.globalInterval > 0 {
			interval = sm.globalInterval
		} else if interval <= 0 {
			interval = 60 // Default to 60 seconds
		}

		method := service.SiteMonitor.Method
		if method == "" {
			method = "HEAD" // Default to HEAD
		}
		timeout := service.SiteMonitor.Timeout
		if timeout <= 0 {
			timeout = 10 // Default to 10 seconds
		}
		url := service.SiteMonitor.URL
		if url == "" && service.Href != "" {
			url = service.Href
		}
		expectedCodes := service.SiteMonitor.ExpectedCodes
		if len(expectedCodes) == 0 {
			expectedCodes = []int{200} // Default to 200 OK
		}

		logging.Info("Starting HTTP monitoring for %s with interval %d seconds", service.Name, interval)

		// Start HTTP monitoring goroutine
		go func() {
			ticker := time.NewTicker(time.Duration(interval) * time.Second)
			defer ticker.Stop()

			// Do an initial check immediately
			sm.checkHTTPService(service.Name, url, method, timeout, expectedCodes, service.SiteMonitor)

			for {
				select {
				case <-ticker.C:
					sm.checkHTTPService(service.Name, url, method, timeout, expectedCodes, service.SiteMonitor)
				case <-stopChan:
					return
				}
			}
		}()
	}
}

// updateServiceStatus updates the status for a service and triggers the callback
func (sm *StatusMonitor) updateServiceStatus(serviceName string, state StatusState, message string) {
	sm.mutex.Lock()
	result, exists := sm.results[serviceName]
	if !exists {
		result = &StatusResult{
			State:       state,
			Message:     message,
			LastChecked: time.Now(),
		}
		sm.results[serviceName] = result

		logging.Info("Status created for %s: State=%s, Message='%s'",
			serviceName, state, message)
		// Trigger callback for new status
		if sm.updateFunc != nil {
			defer sm.updateFunc(serviceName, state, message)
		}
	} else {
		// Update existing result
		oldState := result.State
		oldMessage := result.Message
		result.State = state
		result.Message = message
		result.LastChecked = time.Now()

		logging.Info("Status updated for %s: State=%s, Message='%s'",
			serviceName, state, message)

		// Call the update function if the state or message has changed
		if (oldState != state || oldMessage != message) && sm.updateFunc != nil {
			logging.Info("Status change detected for %s: '%s:%s' -> '%s:%s', triggering callback",
				serviceName, oldState, oldMessage, state, message)
			defer sm.updateFunc(serviceName, state, message)
		}
	}
	sm.mutex.Unlock()
}

// pingService pings a host and updates its status
func (sm *StatusMonitor) pingService(serviceName, host string, count int) {
	logging.Debug("Ping check for %s: Starting ping to %s", serviceName, host)

	var cmd *exec.Cmd
	var pingOpts []string

	if host == "" {
		sm.updateServiceStatus(serviceName, StatusCritical, "No host specified for ping")
		return
	}

	// Different ping parameters for different operating systems
	switch runtime.GOOS {
	case "windows":
		pingOpts = []string{"-n", fmt.Sprintf("%d", count), "-w", "1000"}
	case "darwin", "linux":
		pingOpts = []string{"-c", fmt.Sprintf("%d", count), "-W", "1"}
	default:
		sm.updateServiceStatus(serviceName, StatusWarning, fmt.Sprintf("Ping not supported on %s", runtime.GOOS))
		return
	}

	// Run the ping command
	startTime := time.Now()
	logging.Debug("Ping check for %s: Running ping command with options: %v %s",
		serviceName, pingOpts, host)
	cmd = exec.Command("ping", append(pingOpts, host)...)
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(startTime)

	// Parse the ping output
	pingResults := string(output)
	logging.Debug("Ping check for %s: Raw output: %s", serviceName, pingResults)

	if err != nil {
		// Ping failed
		logging.Error("Ping check for %s: Command failed: %v", serviceName, err)
		sm.updateServiceStatus(serviceName, StatusCritical, fmt.Sprintf("Ping failed: %v", err))
		return
	}

	// Extract response time from ping output
	var avgTime string
	var packetLoss string

	// Different regex for different OS output formats
	switch runtime.GOOS {
	case "windows":
		avgTimeRe := regexp.MustCompile(`Average = (\d+)ms`)
		matches := avgTimeRe.FindStringSubmatch(pingResults)
		if len(matches) > 1 {
			avgTime = matches[1] + "ms"
		}

		lossRe := regexp.MustCompile(`(\d+)% loss`)
		matches = lossRe.FindStringSubmatch(pingResults)
		if len(matches) > 1 {
			packetLoss = matches[1] + "%"
		}
	case "darwin":
		// Mac format: round-trip min/avg/max/stddev = 27.222/32.582/41.860/5.139 ms
		avgTimeRe := regexp.MustCompile(`(?:round-trip|rtt).*?=.*?(\d+\.\d+).*?ms`)
		matches := avgTimeRe.FindStringSubmatch(pingResults)
		if len(matches) > 1 {
			avgTime = matches[1] + "ms"
		}

		lossRe := regexp.MustCompile(`(\d+\.?\d*)% packet loss`)
		matches = lossRe.FindStringSubmatch(pingResults)
		if len(matches) > 1 {
			packetLoss = matches[1] + "%"
		}
	case "linux":
		// Linux format: rtt min/avg/max/mdev = 0.083/0.153/0.223/0.070 ms
		avgTimeRe := regexp.MustCompile(`rtt min/avg/max.*?= [0-9.]+/([0-9.]+)/[0-9.]+`)
		matches := avgTimeRe.FindStringSubmatch(pingResults)
		if len(matches) > 1 {
			avgTime = matches[1] + "ms"
		}

		lossRe := regexp.MustCompile(`(\d+\.?\d*)% packet loss`)
		matches = lossRe.FindStringSubmatch(pingResults)
		if len(matches) > 1 {
			packetLoss = matches[1] + "%"
		}
	}

	// If we couldn't extract avg time, use elapsed time
	if avgTime == "" {
		avgTime = fmt.Sprintf("%.1fms", float64(elapsed.Milliseconds()))
		logging.Warn("Ping check for %s: Could not extract avg time, using elapsed: %s", serviceName, avgTime)
	}

	logging.Debug("Ping check for %s: Completed - avg time: %s, packet loss: %s",
		serviceName, avgTime, packetLoss)

	// Update the status based on packet loss
	if packetLoss == "0%" || packetLoss == "" {
		// No packet loss, service is up
		sm.mutex.Lock()
		if result, exists := sm.results[serviceName]; exists {
			result.ResponseTime = elapsed
		}
		sm.mutex.Unlock()

		sm.updateServiceStatus(serviceName, StatusOK, fmt.Sprintf("Up (%s)", avgTime))
	} else if strings.HasPrefix(packetLoss, "100") {
		// All packets lost, service is down
		sm.updateServiceStatus(serviceName, StatusCritical, fmt.Sprintf("Down (%s loss)", packetLoss))
	} else {
		// Some packets lost, service is having issues
		sm.mutex.Lock()
		if result, exists := sm.results[serviceName]; exists {
			result.ResponseTime = elapsed
		}
		sm.mutex.Unlock()

		// Format with time and packet loss (will be colored differently in UI)
		sm.updateServiceStatus(serviceName, StatusWarning, fmt.Sprintf("Degraded (%s) packet loss: %s", avgTime, packetLoss))
	}
}

// checkHTTPService performs an HTTP request to check a service
func (sm *StatusMonitor) checkHTTPService(serviceName, url, method string, timeoutSec int, expectedCodes []int, config *SiteMonitorConfig) {
	logging.Debug("HTTP check for %s: Starting check for URL %s", serviceName, url)

	if url == "" {
		sm.updateServiceStatus(serviceName, StatusCritical, "No URL specified for HTTP check")
		return
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow redirects but limit to 10
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	// Configure TLS settings if needed
	if config.SkipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	// Create the request
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		logging.Error("HTTP check for %s: Error creating request: %v", serviceName, err)
		sm.updateServiceStatus(serviceName, StatusCritical, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// Add headers if specified
	if config.Headers != nil {
		for key, value := range config.Headers {
			req.Header.Add(key, value)
		}
	}

	// Set User-Agent
	req.Header.Set("User-Agent", "Termdash/1.0")

	// Execute the request
	startTime := time.Now()
	logging.Debug("HTTP check for %s: Sending %s request to %s", serviceName, method, url)
	resp, err := client.Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		logging.Error("HTTP check for %s: Request failed: %v", serviceName, err)
		sm.updateServiceStatus(serviceName, StatusCritical, fmt.Sprintf("Request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	responseTime := elapsed.Milliseconds()
	logging.Debug("HTTP check for %s: Received response code %d in %d ms",
		serviceName, resp.StatusCode, responseTime)

	// Check if status code is in expected codes
	codeIsExpected := false
	for _, expectedCode := range expectedCodes {
		if resp.StatusCode == expectedCode {
			codeIsExpected = true
			break
		}
	}

	if codeIsExpected {
		// Update the response time
		sm.mutex.Lock()
		if result, exists := sm.results[serviceName]; exists {
			result.ResponseTime = elapsed
		}
		sm.mutex.Unlock()

		sm.updateServiceStatus(serviceName, StatusOK, fmt.Sprintf("Up (%d ms)", responseTime))
	} else if resp.StatusCode >= 500 {
		sm.updateServiceStatus(serviceName, StatusCritical, fmt.Sprintf("Server error: %d", resp.StatusCode))
	} else if resp.StatusCode >= 400 {
		sm.updateServiceStatus(serviceName, StatusWarning, fmt.Sprintf("Client error: %d", resp.StatusCode))
	} else {
		sm.updateServiceStatus(serviceName, StatusWarning, fmt.Sprintf("Unexpected response: %d", resp.StatusCode))
	}
}

// Basic container information for status display
type dockerContainer struct {
	ID     string
	Name   string
	Image  string
	Status string
	Health string
}

// createDockerClient creates a Docker client based on the config
func createDockerClient(config *DockerConfig) (*client.Client, error) {
	var cli *client.Client
	var err error

	// Create client based on configuration
	if config.Socket != "" {
		// Socket connection
		logging.Debug("Using Docker socket: %s", config.Socket)
		cli, err = client.NewClientWithOpts(
			client.WithHost(fmt.Sprintf("unix://%s", config.Socket)),
			client.WithAPIVersionNegotiation(),
		)
	} else if config.Host != "" {
		// Remote host connection
		hostArg := config.Host
		if config.Port > 0 {
			hostArg = fmt.Sprintf("%s:%d", config.Host, config.Port)
		}
		logging.Debug("Using Docker remote host: %s", hostArg)
		cli, err = client.NewClientWithOpts(
			client.WithHost(fmt.Sprintf("tcp://%s", hostArg)),
			client.WithAPIVersionNegotiation(),
		)
	} else {
		// Default connection
		logging.Debug("Using default Docker connection")
		cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return cli, nil
}

// checkDockerContainers checks the status of docker containers
func (sm *StatusMonitor) checkDockerContainers(config *DockerConfig) error {
	logging.Debug("Checking Docker containers status...")

	// Create Docker client
	cli, err := createDockerClient(config)
	if err != nil {
		logging.Error("ERROR: Failed to create Docker client: %v", err)
		return err
	}
	defer cli.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List containers
	containerList, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		logging.Error("ERROR: Docker container list error: %v", err)
		return err
	}

	// Convert to our internal container representation
	var containers []dockerContainer
	for _, c := range containerList {
		// Container name comes with a leading slash we need to remove
		name := c.Names[0]
		if strings.HasPrefix(name, "/") {
			name = name[1:]
		}

		// Determine health status if available
		health := ""
		if c.State == "running" && c.Status != "" {
			if strings.Contains(c.Status, "(healthy)") {
				health = "healthy"
			} else if strings.Contains(c.Status, "(unhealthy)") {
				health = "unhealthy"
			} else if strings.Contains(c.Status, "(health: starting)") {
				health = "starting"
			}
		}

		containers = append(containers, dockerContainer{
			ID:     c.ID,
			Name:   name,
			Image:  c.Image,
			Status: c.Status,
			Health: health,
		})
	}

	logging.Debug("Found %d Docker containers", len(containers))

	// Create a map of Docker services by container name for easier lookup
	dockerServices := make(map[string][]*Service)
	for serviceName, service := range sm.services {
		if service.Container != "" {
			logging.Debug("Service '%s' references container: '%s', server: '%s'",
				serviceName, service.Container, service.Server)

			// Add this service to the map by container name
			dockerServices[service.Container] = append(dockerServices[service.Container], service)
		}
	}

	// Log the number of services with container references
	logging.Debug("Found %d services with container references", len(dockerServices))

	// Process each container
	processedServices := make(map[string]bool)
	processedContainers := make(map[string]bool)

	logging.Debug("Container names found by Docker API:")
	for _, container := range containers {
		logging.Debug("- Container: '%s', Status: '%s'", container.Name, container.Status)
	}

	// First attempt exact matches
	for _, container := range containers {
		// Find services with exact container name match
		if services, found := dockerServices[container.Name]; found {
			for _, service := range services {
				logging.Debug("Exact match: Container '%s' matches service '%s'",
					container.Name, service.Name)
				sm.updateDockerServiceStatus(service.Name, service, container)
				processedServices[service.Name] = true
				processedContainers[container.Name] = true
			}
		}
	}

	// Then try substring matches for remaining services
	for _, container := range containers {
		for containerName, services := range dockerServices {
			// Skip if we've already processed all services for this container name
			if allProcessed(services, processedServices) {
				continue
			}

			// Check for substring match
			if strings.Contains(container.Name, containerName) ||
				strings.Contains(containerName, container.Name) {
				for _, service := range services {
					if !processedServices[service.Name] {
						logging.Debug("Substring match: Container '%s' matches service '%s' (container='%s')",
							container.Name, service.Name, containerName)
						sm.updateDockerServiceStatus(service.Name, service, container)
						processedServices[service.Name] = true
						processedContainers[container.Name] = true
					}
				}
			}
		}
	}

	// Mark any remaining services as not found
	for containerName, services := range dockerServices {
		for _, service := range services {
			if !processedServices[service.Name] {
				logging.Debug("No container found for service '%s' (container='%s')",
					service.Name, containerName)
				sm.updateServiceStatus(service.Name, StatusCritical, "Container not found")
			}
		}
	}

	// Autodiscovery: Check for containers with homepage labels that aren't tracked yet
	sm.discoverContainersWithLabels(containers, processedContainers, config)

	return nil
}

// Helper function to check if all services have been processed
func allProcessed(services []*Service, processedServices map[string]bool) bool {
	for _, service := range services {
		if !processedServices[service.Name] {
			return false
		}
	}
	return true
}

// discoverContainersWithLabels discovers containers with homepage labels and adds them as services
func (sm *StatusMonitor) discoverContainersWithLabels(containers []dockerContainer, processedContainers map[string]bool, config *DockerConfig) {
	logging.Debug("Checking for containers with homepage labels...")

	// Skip autodiscovery if disabled in config
	if config != nil && config.DisableAutodiscovery {
		logging.Debug("Autodiscovery disabled in config, skipping")
		return
	}

	// Count discovered services for logging
	discoveredCount := 0

	// Create Docker client for label inspection
	cli, err := createDockerClient(config)
	if err != nil {
		logging.Error("ERROR: Failed to create Docker client for autodiscovery: %v", err)
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logging.Debug("Examining %d containers for autodiscovery", len(containers))
	for containerIndex, container := range containers {
		// Skip containers we've already matched
		if processedContainers[container.Name] {
			logging.Debug("Container '%s' already processed, skipping", container.Name)
			continue
		}

		logging.Debug("Inspecting container %d/%d: '%s' for homepage labels",
			containerIndex+1, len(containers), container.Name)

		// Inspect container to get labels
		containerInfo, err := cli.ContainerInspect(ctx, container.ID)
		if err != nil {
			logging.Error("ERROR: Failed to inspect container %s: %v", container.Name, err)
			continue
		}

		// Check for homepage labels
		homepageLabels := false
		labels := containerInfo.Config.Labels
		for key, value := range labels {
			if strings.HasPrefix(key, "homepage.") {
				homepageLabels = true
				logging.Debug("Found homepage label on container '%s': %s=%s",
					container.Name, key, value)
			}
		}

		// Skip if no homepage labels found
		if !homepageLabels {
			logging.Debug("No homepage labels found on container '%s'", container.Name)
			continue
		}

		// Extract info from the labels
		name := container.Name
		if val, ok := labels["homepage.name"]; ok && val != "" {
			name = val
		}

		group := "Docker"
		if val, ok := labels["homepage.group"]; ok && val != "" {
			group = val
		}

		// Add service to the monitor if not already being monitored
		if _, exists := sm.services[name]; !exists {
			logging.Debug("Creating service for container '%s' with name '%s'",
				container.Name, name)

			// Create a new service from the autodiscovered container
			service := &Service{
				Name:        name,
				Container:   container.Name,
				Description: fmt.Sprintf("Image: %s", container.Image),
				Server:      "local-docker", // Assume local
			}

			if val, ok := labels["homepage.description"]; ok {
				service.Description = val
			}

			// Set the href if provided
			if val, ok := labels["homepage.href"]; ok {
				service.Href = val
			}

			// Add to services map and results
			sm.mutex.Lock()
			sm.services[service.Name] = service
			sm.results[service.Name] = &StatusResult{
				State:       StatusUnknown,
				Message:     "Discovered service",
				LastChecked: time.Now(),
			}
			sm.mutex.Unlock()

			// Call autodiscover to add to UI
			sm.autodiscoverService(service, group)

			// Update the status immediately
			sm.updateDockerServiceStatus(service.Name, service, container)

			discoveredCount++
		}
	}

	logging.Debug("Docker autodiscovery completed - found %d containers with homepage labels", discoveredCount)
}

// autodiscoverService adds a service discovered from container labels
func (sm *StatusMonitor) autodiscoverService(service *Service, groupName string) {
	logging.Info("Auto-discovering service '%s' in group '%s'", service.Name, groupName)

	// Add service to status monitor
	sm.AddService(service)

	// Add to dynamic service group in UI
	logging.Debug("Adding '%s' to dynamic service group '%s'", service.Name, groupName)
	//AddDynamicServiceGroup(groupName, service)

	// Log after adding to verify it was added
	logging.Debug("Service '%s' has been added to group '%s' and should appear in the UI",
		service.Name, groupName)
}

// updateDockerServiceStatus updates the status of a service based on Docker container state
func (sm *StatusMonitor) updateDockerServiceStatus(serviceName string, service *Service, container dockerContainer) {
	// Determine status based on container state and health
	state := StatusUnknown
	var message string

	// Check health status first
	if container.Health == "healthy" {
		state = StatusOK
		message = fmt.Sprintf("Up (%s)", container.Health)
	} else if container.Health == "unhealthy" {
		state = StatusCritical
		message = fmt.Sprintf("Unhealthy (%s)", container.Status)
	} else if strings.HasPrefix(container.Status, "Up") {
		// Container is running but no health check or starting
		state = StatusOK
		message = fmt.Sprintf("Running (%s)", container.Status)
	} else if strings.HasPrefix(container.Status, "Exited (0)") {
		// Exited with code 0 is usually OK for one-off containers
		state = StatusWarning
		message = fmt.Sprintf("Exited (%s)", container.Status)
	} else if strings.HasPrefix(container.Status, "Exited") ||
		strings.Contains(container.Status, "Dead") {
		state = StatusCritical
		message = fmt.Sprintf("Stopped (%s)", container.Status)
	} else if strings.Contains(container.Status, "Restarting") {
		state = StatusWarning
		message = fmt.Sprintf("Restarting (%s)", container.Status)
	} else {
		// Unknown status
		message = fmt.Sprintf("Unknown (%s)", container.Status)
	}

	// Update the status for this service
	logging.Debug("Updating status for %s to %s: %s", serviceName, state, message)
	sm.updateServiceStatus(serviceName, state, message)
}

// GetServiceStatusString returns a string representation of the service status
func (sm *StatusMonitor) GetServiceStatusString(serviceName string) string {
	result := sm.GetStatus(serviceName)
	if result.State == StatusUnknown && result.LastChecked.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s %s", string(result.State), result.Message)
}

// Add a method to set the global interval
func (sm *StatusMonitor) SetGlobalInterval(seconds int) {
	if seconds > 0 {
		logging.Info("Setting global status check interval to %d seconds", seconds)
		sm.globalInterval = seconds
	}
}

// RunInitialDockerDiscovery runs autodiscovery immediately during startup
func (sm *StatusMonitor) RunInitialDockerDiscovery(config *DockerConfig) error {
	if config == nil {
		return fmt.Errorf("docker config is nil")
	}

	logging.Info("Running initial Docker container discovery...")

	// Perform an immediate container check to discover services
	if err := sm.checkDockerContainers(config); err != nil {
		return fmt.Errorf("failed to check docker containers: %w", err)
	}

	logging.Info("Initial Docker discovery completed")
	return nil
}

// GetStatusMonitor returns the global status monitor instance
func GetStatusMonitor() *StatusMonitor {
	return statusMonitor
}
