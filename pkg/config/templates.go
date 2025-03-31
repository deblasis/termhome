package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Templates for configuration files

// SettingsTemplate is the template for settings.yaml
const SettingsTemplate = `# Termhome Settings
title: Termhome Dashboard
status:
  checkInterval: 10 # Default status check interval in seconds, overrides individual services if set
`

// ServicesTemplate is the template for services.yaml
const ServicesTemplate = `# Termhome Services Configuration

Applications:
  - name: GitHub
    href: https://github.com
    description: Where the world builds software
    siteMonitor:
      url: https://github.com
      method: HEAD
      timeout: 5
      interval: 60
      expectedCodes: [200, 301, 302]
  - name: ChatGPT
    href: https://chatgpt.com
    description: The AI chatbot
    ping:
      host: chatgpt.com
      count: 3
      interval: 60

Development Tools:
  - name: V0
    href: https://v0.dev
    siteMonitor:
      url: https://v0.dev
      method: GET
      timeout: 10
      interval: 60
      expectedCodes: [429]
      headers:
        User-Agent: "Termhome/1.0"

Monitoring:
  - name: Local Network
    description: Local network status
    ping:
      host: 192.168.1.1
      interval: 30
`

// BookmarksTemplate is the template for bookmarks.yaml
const BookmarksTemplate = `# Termhome Bookmarks Configuration

Documentation:
  - name: Termhome Docs
    href: https://github.com/deblasis/termhome
    description: Termhome documentation

Search Engines:
  - name: Google
    abbr: G
    href: https://google.com
    description: Google Search
  - name: DuckDuckGo
    abbr: DDG
    href: https://duckduckgo.com
    description: Privacy-focused search engine
  - name: Perplexity
    abbr: P
    href: https://perplexity.com
    description: AI-powered search engine
`

// DockerTemplate is the template for docker.yaml
const DockerTemplate = `# Direct Docker socket connection
local-docker:
  socket: /var/run/docker.sock
  interval: 10  # Check every 10 seconds

# Connection via Docker proxy (uncomment to enable)
# docker-proxy:
#   host: dockerproxy
#   port: 2375
`

// GetAllTemplates returns a map of filenames to their template content
func GetAllTemplates() map[string]string {
	return map[string]string{
		"settings.yaml":  SettingsTemplate,
		"services.yaml":  ServicesTemplate,
		"bookmarks.yaml": BookmarksTemplate,
		"docker.yaml":    DockerTemplate,
	}
}

// InitializeConfigDir creates a config directory with example configuration files
func InitializeConfigDir(configDir string) error {
	// Create the config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Get all templates
	templates := GetAllTemplates()

	// Write the files
	for filename, content := range templates {
		filePath := filepath.Join(configDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filePath, err)
		}
	}

	return nil
}
