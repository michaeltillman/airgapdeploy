// Package config defines the single Config model that drives every generated
// artifact, along with its defaults, validation, and YAML load/save helpers.
package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Default versions target k0rdent Enterprise 1.4.0. Confirm exact versions
// against https://get.mirantis.com/k0rdent-enterprise before a real run.
const (
	DefaultKcmVersion    = "1.4.0"
	DefaultK0sVersion    = "v1.35.4+k0s.0"
	DefaultSkopeoVersion = "v1.17.0"
	DefaultNginxImage    = "nginx:1.30.2"
	DefaultBusyboxImage  = "busybox:1.36.1"
	DefaultNamespace     = "kcm-system"
	DefaultBundleBaseURL = "https://get.mirantis.com/k0rdent-enterprise"
	DefaultOutputDir     = "artifacts"
	DefaultFileserverNS  = "k0s-fileserver"
	DefaultPVCSize       = "10Gi"
)

// Config is the complete set of inputs the operator provides. It is the single
// source of truth passed to every template.
type Config struct {
	// Versions
	KcmVersion    string   `yaml:"kcmVersion" json:"kcmVersion"`
	K0sVersion    string   `yaml:"k0sVersion" json:"k0sVersion"`
	SkopeoVersion string   `yaml:"skopeoVersion" json:"skopeoVersion"`
	Architectures []string `yaml:"architectures" json:"architectures"`

	// Registry (non-authenticated OCI mirror, e.g. Harbor)
	RegistryHost string `yaml:"registryHost" json:"registryHost"` // e.g. registry.local
	// RegistryProject is the path segment images/charts live under.
	RegistryProject string `yaml:"registryProject" json:"registryProject"` // default: k0rdent-enterprise

	// k0s binary HTTP fileserver
	FileserverURL string `yaml:"fileserverURL" json:"fileserverURL"` // e.g. http://binary.local/k0rdent-enterprise

	// In-cluster nginx fileserver (used to serve the k0s binary)
	FileserverNamespace string `yaml:"fileserverNamespace" json:"fileserverNamespace"`
	StorageClass        string `yaml:"storageClass" json:"storageClass"`
	PVCSize             string `yaml:"pvcSize" json:"pvcSize"`
	NginxImage          string `yaml:"nginxImage" json:"nginxImage"`     // relative to RegistryHost
	BusyboxImage        string `yaml:"busyboxImage" json:"busyboxImage"` // relative to Registry

	// Install target
	Namespace string `yaml:"namespace" json:"namespace"`

	// Sources / output
	BundleBaseURL string `yaml:"bundleBaseURL" json:"bundleBaseURL"`
	OutputDir     string `yaml:"outputDir" json:"outputDir"`

	// TLS. TLSMode selects how the registry/fileserver certificates are trusted:
	// "none" (plain HTTP or publicly-trusted), "self-signed", or "custom-ca".
	// For the two cert modes the operator supplies a cert file at CACertPath;
	// the generated artifacts create CACertSecretName from it before install.
	TLSMode          string `yaml:"tlsMode" json:"tlsMode"`
	CACertPath       string `yaml:"caCertPath" json:"caCertPath"`
	CACertSecretName string `yaml:"caCertSecretName" json:"caCertSecretName"`
}

// TLS modes.
const (
	TLSNone       = "none"
	TLSSelfSigned = "self-signed"
	TLSCustomCA   = "custom-ca"
)

// Default returns a Config populated with sensible airgap defaults.
func Default() *Config {
	return &Config{
		KcmVersion:          DefaultKcmVersion,
		K0sVersion:          DefaultK0sVersion,
		SkopeoVersion:       DefaultSkopeoVersion,
		Architectures:       []string{"amd64", "arm64"},
		RegistryHost:        "registry.local",
		RegistryProject:     "k0rdent-enterprise",
		FileserverURL:       "http://binary.local/k0rdent-enterprise",
		FileserverNamespace: DefaultFileserverNS,
		StorageClass:        "",
		PVCSize:             DefaultPVCSize,
		NginxImage:          DefaultNginxImage,
		BusyboxImage:        DefaultBusyboxImage,
		Namespace:           DefaultNamespace,
		BundleBaseURL:       DefaultBundleBaseURL,
		OutputDir:           DefaultOutputDir,
		TLSMode:             TLSNone,
		CACertPath:          "./ca.crt",
		CACertSecretName:    "registry-ca-cert",
	}
}

// UsesCustomTLS reports whether a CA certificate secret must be created and
// referenced (true for both self-signed and custom-ca modes).
func (c *Config) UsesCustomTLS() bool {
	return c.TLSMode == TLSSelfSigned || c.TLSMode == TLSCustomCA
}

// FileserverIsHTTPS reports whether the k0s fileserver URL uses HTTPS, in which
// case the k0s download also needs the CA secret.
func (c *Config) FileserverIsHTTPS() bool {
	return strings.HasPrefix(c.FileserverURL, "https://")
}

// TLSModeLabel returns a human-readable description of the configured TLS mode.
func (c *Config) TLSModeLabel() string {
	switch c.TLSMode {
	case TLSSelfSigned:
		return "self-signed certificate"
	case TLSCustomCA:
		return "certificate signed by a custom/internal CA"
	default:
		return "no custom CA (plain HTTP or publicly-trusted TLS)"
	}
}

// Registry returns the fully-qualified registry base, e.g.
// "registry.local/k0rdent-enterprise".
func (c *Config) Registry() string {
	return c.RegistryHost + "/" + c.RegistryProject
}

// ChartsRepoURL returns the OCI URL for the k0rdent-enterprise charts.
func (c *Config) ChartsRepoURL() string {
	return "oci://" + c.Registry() + "/charts"
}

// BundleName returns the airgap bundle archive filename.
func (c *Config) BundleName() string {
	return fmt.Sprintf("airgap-bundle-%s.tar.gz", c.KcmVersion)
}

// K0sBinaryPrefix returns the k0s binary name without the architecture suffix,
// e.g. "k0s-v1.35.4+k0s.0".
func (c *Config) K0sBinaryPrefix() string {
	return "k0s-" + c.K0sVersion
}

// K0sBinaryName returns the k0s binary filename for a given architecture.
// The binary MUST NOT be renamed or child cluster deployment fails.
func (c *Config) K0sBinaryName(arch string) string {
	return fmt.Sprintf("k0s-%s-%s", c.K0sVersion, arch)
}

// PrimaryK0sBinary returns the k0s binary name for the first configured
// architecture, used as the default binary to serve from the fileserver.
func (c *Config) PrimaryK0sBinary() string {
	arch := "amd64"
	if len(c.Architectures) > 0 {
		arch = c.Architectures[0]
	}
	return c.K0sBinaryName(arch)
}

// BusyboxImageRef returns the busybox image reference under the private registry.
func (c *Config) BusyboxImageRef() string {
	return c.Registry() + "/" + c.BusyboxImage
}

// NginxImageRef returns the nginx image reference (served from the registry host root).
func (c *Config) NginxImageRef() string {
	return c.RegistryHost + "/" + c.NginxImage
}

var (
	hostRe    = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?(:[0-9]{1,5})?$`)
	versionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+`)
	dnsLabel  = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
)

// Validate reports the first configuration problem, or nil if the Config is usable.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.KcmVersion) == "" || !versionRe.MatchString(c.KcmVersion) {
		return fmt.Errorf("kcmVersion %q is not a valid semantic version (e.g. 1.4.0)", c.KcmVersion)
	}
	if strings.TrimSpace(c.K0sVersion) == "" || !strings.HasPrefix(c.K0sVersion, "v") {
		return fmt.Errorf("k0sVersion %q must look like v1.35.4+k0s.0", c.K0sVersion)
	}
	if len(c.Architectures) == 0 {
		return fmt.Errorf("at least one architecture is required (e.g. amd64)")
	}
	for _, a := range c.Architectures {
		if a != "amd64" && a != "arm64" {
			return fmt.Errorf("unsupported architecture %q (expected amd64 or arm64)", a)
		}
	}
	if !hostRe.MatchString(c.RegistryHost) {
		return fmt.Errorf("registryHost %q is not a valid host[:port]", c.RegistryHost)
	}
	if strings.TrimSpace(c.RegistryProject) == "" {
		return fmt.Errorf("registryProject must not be empty")
	}
	if err := validateURL(c.FileserverURL, "fileserverURL"); err != nil {
		return err
	}
	if err := validateURL(c.BundleBaseURL, "bundleBaseURL"); err != nil {
		return err
	}
	if !dnsLabel.MatchString(c.Namespace) {
		return fmt.Errorf("namespace %q is not a valid DNS label", c.Namespace)
	}
	if !dnsLabel.MatchString(c.FileserverNamespace) {
		return fmt.Errorf("fileserverNamespace %q is not a valid DNS label", c.FileserverNamespace)
	}
	if strings.TrimSpace(c.OutputDir) == "" {
		return fmt.Errorf("outputDir must not be empty")
	}
	switch c.TLSMode {
	case TLSNone, TLSSelfSigned, TLSCustomCA:
	default:
		return fmt.Errorf("tlsMode %q must be one of none, self-signed, custom-ca", c.TLSMode)
	}
	if c.UsesCustomTLS() {
		if strings.TrimSpace(c.CACertSecretName) == "" {
			return fmt.Errorf("caCertSecretName is required for tlsMode %q", c.TLSMode)
		}
		if strings.TrimSpace(c.CACertPath) == "" {
			return fmt.Errorf("caCertPath is required for tlsMode %q", c.TLSMode)
		}
	}
	return nil
}

func validateURL(raw, field string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s %q is not a valid URL: %w", field, raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s %q must use http or https", field, raw)
	}
	if u.Host == "" {
		return fmt.Errorf("%s %q is missing a host", field, raw)
	}
	return nil
}

var quantityRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(Ki|Mi|Gi|Ti|Pi|K|M|G|T|P)?$`)

// ValidateAll checks every field and returns a map of field name -> human-readable
// error message for all problems found (not just the first). An empty map means
// the config is valid. Field names match the JSON/form field names so the UI can
// highlight the offending input.
func (c *Config) ValidateAll() map[string]string {
	e := map[string]string{}

	if strings.TrimSpace(c.KcmVersion) == "" {
		e["kcmVersion"] = "k0rdent Enterprise version is required (e.g. 1.4.0)"
	} else if !versionRe.MatchString(c.KcmVersion) {
		e["kcmVersion"] = fmt.Sprintf("%q is not a valid version (expected e.g. 1.4.0)", c.KcmVersion)
	}

	if strings.TrimSpace(c.K0sVersion) == "" {
		e["k0sVersion"] = "k0s version is required (e.g. v1.35.4+k0s.0)"
	} else if !strings.HasPrefix(c.K0sVersion, "v") {
		e["k0sVersion"] = fmt.Sprintf("%q must look like v1.35.4+k0s.0", c.K0sVersion)
	}

	if len(c.Architectures) == 0 {
		e["architectures"] = "select at least one architecture"
	} else {
		for _, a := range c.Architectures {
			if a != "amd64" && a != "arm64" {
				e["architectures"] = fmt.Sprintf("unsupported architecture %q (amd64 or arm64)", a)
			}
		}
	}

	if strings.TrimSpace(c.RegistryHost) == "" {
		e["registryHost"] = "registry host is required (e.g. registry.local)"
	} else if !hostRe.MatchString(c.RegistryHost) {
		e["registryHost"] = fmt.Sprintf("%q is not a valid host[:port]", c.RegistryHost)
	}

	if strings.TrimSpace(c.RegistryProject) == "" {
		e["registryProject"] = "registry project/path is required"
	}

	if strings.TrimSpace(c.FileserverURL) == "" {
		e["fileserverURL"] = "fileserver URL is required (e.g. http://binary.local/k0rdent-enterprise)"
	} else if err := validateURL(c.FileserverURL, "fileserverURL"); err != nil {
		e["fileserverURL"] = shortURLErr(c.FileserverURL)
	}

	if strings.TrimSpace(c.BundleBaseURL) == "" {
		e["bundleBaseURL"] = "bundle base URL is required"
	} else if err := validateURL(c.BundleBaseURL, "bundleBaseURL"); err != nil {
		e["bundleBaseURL"] = shortURLErr(c.BundleBaseURL)
	}

	if !dnsLabel.MatchString(c.Namespace) {
		e["namespace"] = fmt.Sprintf("%q is not a valid namespace (lowercase DNS label)", c.Namespace)
	}
	if !dnsLabel.MatchString(c.FileserverNamespace) {
		e["fileserverNamespace"] = fmt.Sprintf("%q is not a valid namespace (lowercase DNS label)", c.FileserverNamespace)
	}

	if strings.TrimSpace(c.OutputDir) == "" {
		e["outputDir"] = "output directory is required"
	}

	if c.PVCSize != "" && !quantityRe.MatchString(c.PVCSize) {
		e["pvcSize"] = fmt.Sprintf("%q is not a valid size (e.g. 10Gi)", c.PVCSize)
	}

	switch c.TLSMode {
	case TLSNone, TLSSelfSigned, TLSCustomCA:
	default:
		e["tlsMode"] = fmt.Sprintf("%q must be none, self-signed, or custom-ca", c.TLSMode)
	}
	if c.UsesCustomTLS() {
		if strings.TrimSpace(c.CACertPath) == "" {
			e["caCertPath"] = "certificate file path is required for this TLS mode"
		}
		if strings.TrimSpace(c.CACertSecretName) == "" {
			e["caCertSecretName"] = "CA secret name is required for this TLS mode"
		} else if !dnsLabel.MatchString(c.CACertSecretName) {
			e["caCertSecretName"] = fmt.Sprintf("%q is not a valid secret name (lowercase DNS label)", c.CACertSecretName)
		}
	}

	return e
}

func shortURLErr(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Sprintf("%q is not a valid URL", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Sprintf("%q must start with http:// or https://", raw)
	}
	return fmt.Sprintf("%q is not a valid URL", raw)
}

// Normalize trims whitespace and applies defaults for any empty optional fields.
// It is called before validation so partially-filled configs behave predictably.
func (c *Config) Normalize() {
	c.KcmVersion = strings.TrimSpace(c.KcmVersion)
	c.K0sVersion = strings.TrimSpace(c.K0sVersion)
	c.RegistryHost = strings.TrimSpace(strings.TrimSuffix(c.RegistryHost, "/"))
	c.RegistryProject = strings.Trim(strings.TrimSpace(c.RegistryProject), "/")
	c.FileserverURL = strings.TrimSpace(strings.TrimSuffix(c.FileserverURL, "/"))
	c.BundleBaseURL = strings.TrimSpace(strings.TrimSuffix(c.BundleBaseURL, "/"))
	c.Namespace = strings.TrimSpace(c.Namespace)
	c.FileserverNamespace = strings.TrimSpace(c.FileserverNamespace)
	c.OutputDir = strings.TrimSpace(c.OutputDir)

	d := Default()
	if c.SkopeoVersion == "" {
		c.SkopeoVersion = d.SkopeoVersion
	}
	if c.RegistryProject == "" {
		c.RegistryProject = d.RegistryProject
	}
	if c.FileserverNamespace == "" {
		c.FileserverNamespace = d.FileserverNamespace
	}
	if c.PVCSize == "" {
		c.PVCSize = d.PVCSize
	}
	if c.NginxImage == "" {
		c.NginxImage = d.NginxImage
	}
	if c.BusyboxImage == "" {
		c.BusyboxImage = d.BusyboxImage
	}
	if c.Namespace == "" {
		c.Namespace = d.Namespace
	}
	if c.BundleBaseURL == "" {
		c.BundleBaseURL = d.BundleBaseURL
	}
	if c.OutputDir == "" {
		c.OutputDir = d.OutputDir
	}
	if len(c.Architectures) == 0 {
		c.Architectures = d.Architectures
	}
	if strings.TrimSpace(c.TLSMode) == "" {
		c.TLSMode = TLSNone
	}
	if c.CACertPath == "" {
		c.CACertPath = d.CACertPath
	}
	if c.CACertSecretName == "" {
		c.CACertSecretName = d.CACertSecretName
	}
}

// Load reads a YAML config file, filling defaults for any omitted fields.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := Default()
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	c.Normalize()
	return c, nil
}

// MarshalYAML renders the config as YAML bytes.
func (c *Config) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(c)
}

// Save writes the config as YAML to path.
func (c *Config) Save(path string) error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
