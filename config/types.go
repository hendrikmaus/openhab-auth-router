package config

// Main is the root level of the config
type Main struct {
	Passthrough bool             `yaml:"passthrough"`
	Users       map[string]*User `yaml:"users"`
}

// User configures each users access
type User struct {
	Entrypoint string           `yaml:"entrypoint"`
	Sitemaps   Sitemap          `yaml:"sitemaps"`
	Paths      map[string]*Path `yaml:"paths"`
}

// UserName extends User by the name property
type UserName struct {
	Name string
	*User
}

// Sitemap defines defaults, allowed
type Sitemap struct {
	Default string   `yaml:"default"`
	Allowed []string `yaml:"allowed"`
}

// Path a user can or cannot access
type Path struct {
	Allowed bool `yaml:"allowed"`
}

// PathName extends path to get the actual path whic is the map key
type PathName struct {
	Name string
	*Path
}
