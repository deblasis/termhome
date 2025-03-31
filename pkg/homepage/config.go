package homepage

// Settings represents the structure of the settings.yaml file.
// We only care about the title for the MVP.
type Settings struct {
	Title  string         `yaml:"title"`
	Status StatusSettings `yaml:"status"`
	// Other settings are ignored for now (Layout, Theme, etc.)
}

// StatusSettings holds global status monitoring settings
type StatusSettings struct {
	CheckInterval int `yaml:"checkInterval"` // Global status check interval in seconds
}

// PingConfig holds configuration for ICMP ping monitoring
type PingConfig struct {
	Host     string `yaml:"host"`     // Required: Host to ping
	Count    int    `yaml:"count"`    // Optional: Number of pings to send (default: 3)
	Interval int    `yaml:"interval"` // Optional: Ping interval in seconds (default: 60)
}

// SiteMonitorConfig holds configuration for HTTP site monitoring
type SiteMonitorConfig struct {
	URL           string            `yaml:"url"`           // Required: URL to monitor
	Method        string            `yaml:"method"`        // Optional: HTTP method (default: HEAD)
	Timeout       int               `yaml:"timeout"`       // Optional: Request timeout in seconds (default: 10)
	Interval      int               `yaml:"interval"`      // Optional: Check interval in seconds (default: 60)
	ExpectedCodes []int             `yaml:"expectedCodes"` // Optional: HTTP codes to consider "up" (default: [200])
	Headers       map[string]string `yaml:"headers"`       // Optional: Headers to include in the request
	SkipVerify    bool              `yaml:"skipVerify"`    // Optional: Skip TLS certificate verification
}

// StatusStyle defines custom styling for status indicators
type StatusStyle struct {
	Icon  string `yaml:"icon"`  // Custom icon for this status
	Color string `yaml:"color"` // Custom color for this status
}

// Service represents a single service entry within a group in services.yaml.
type Service struct {
	Name          string                 `yaml:"name"`          // Required: Name of the service
	Href          string                 `yaml:"href"`          // Optional: URL for the service
	Description   string                 `yaml:"description"`   // Optional: Description shown below the name
	Status        string                 `yaml:"status"`        // Optional: Custom field for static text status
	Ping          *PingConfig            `yaml:"ping"`          // Optional: ICMP ping configuration
	SiteMonitor   *SiteMonitorConfig     `yaml:"siteMonitor"`   // Optional: HTTP site monitoring configuration
	StatusStyle   map[string]StatusStyle `yaml:"statusStyle"`   // Optional: Custom styling for status indicators
	DisableStatus bool                   `yaml:"disableStatus"` // Optional: Disable status monitoring for this service
	Server        string                 `yaml:"server"`        // Optional: Docker server reference
	Container     string                 `yaml:"container"`     // Optional: Docker container name
	ShowStats     bool                   `yaml:"showStats"`     // Optional: Show Docker stats
	// Icon, Widget, etc. are ignored for now
}

// ServiceGroup represents a group of services in services.yaml.
// The key in the YAML map becomes the group name.
type ServiceGroup struct {
	Name     string
	Services []*Service // Slice of services in this group
}

// ServicesConfig represents the structure of the services.yaml file.
// It's a map where keys are group names and values are lists of services.
type ServicesConfig map[string][]*Service

// Bookmark represents a single bookmark entry within a group in bookmarks.yaml.
type Bookmark struct {
	Name        string `yaml:"name"`        // Optional: Display name of the bookmark
	Abbr        string `yaml:"abbr"`        // Optional: Abbreviation, used if name is missing
	Href        string `yaml:"href"`        // Required: URL for the bookmark
	Description string `yaml:"description"` // Optional: Description shown on hover/tooltip (or below name)
	// Icon is ignored for now
}

// BookmarkGroup represents a group of bookmarks in bookmarks.yaml.
// The key in the YAML map becomes the group name.
type BookmarkGroup struct {
	Name      string
	Bookmarks []*Bookmark // Slice of bookmarks in this group
}

// BookmarksConfig represents the structure of the bookmarks.yaml file.
// It's a map where keys are group names and values are lists of bookmarks.
type BookmarksConfig map[string][]*Bookmark

// DockerConfig holds configuration for Docker container monitoring
type DockerConfig struct {
	Socket               string   `yaml:"socket"`               // Unix socket path
	Host                 string   `yaml:"host"`                 // Remote host
	Port                 int      `yaml:"port"`                 // Remote port
	Interval             int      `yaml:"interval"`             // Check interval in seconds
	Includes             []string `yaml:"includes"`             // Container name patterns to include
	Excludes             []string `yaml:"excludes"`             // Container name patterns to exclude
	DisableAutodiscovery bool     `yaml:"disableAutodiscovery"` // Disable auto-discovery based on labels
}

// DockerContainer represents a single Docker container
type DockerContainer struct {
	ID      string   `yaml:"id"`      // Container ID
	Name    string   `yaml:"name"`    // Container name
	Image   string   `yaml:"image"`   // Image name
	Status  string   `yaml:"status"`  // Running status
	Created int64    `yaml:"created"` // Creation timestamp
	Ports   []string `yaml:"ports"`   // Exposed ports
}
