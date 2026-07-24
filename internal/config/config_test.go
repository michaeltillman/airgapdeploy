package config

import "testing"

func TestDefaultIsValid(t *testing.T) {
	c := Default()
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("default config should validate, got: %v", err)
	}
}

func TestDerivedValues(t *testing.T) {
	c := Default()
	c.RegistryHost = "reg.local"
	c.RegistryProject = "k0rdent-enterprise"
	if got := c.Registry(); got != "reg.local/k0rdent-enterprise" {
		t.Errorf("Registry() = %q", got)
	}
	if got := c.ChartsRepoURL(); got != "oci://reg.local/k0rdent-enterprise/charts" {
		t.Errorf("ChartsRepoURL() = %q", got)
	}
	c.K0sVersion = "v1.35.4+k0s.0"
	c.Architectures = []string{"arm64", "amd64"}
	if got := c.PrimaryK0sBinary(); got != "k0s-v1.35.4+k0s.0-arm64" {
		t.Errorf("PrimaryK0sBinary() = %q", got)
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	// These mutations produce invalid configs even after Normalize().
	cases := map[string]func(*Config){
		"bad kcm version": func(c *Config) { c.KcmVersion = "latest" },
		"bad k0s version": func(c *Config) { c.K0sVersion = "1.35.4" },
		"bad arch":        func(c *Config) { c.Architectures = []string{"ppc64"} },
		"bad registry":    func(c *Config) { c.RegistryHost = "http://bad host/" },
		"bad fileserver":  func(c *Config) { c.FileserverURL = "ftp://x" },
		"bad namespace":   func(c *Config) { c.Namespace = "Bad_NS" },
	}
	for name, mutate := range cases {
		c := Default()
		mutate(c)
		c.Normalize()
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}

	// Fields that Validate rejects only when Normalize does not run.
	rawEmpty := Default()
	rawEmpty.OutputDir = ""
	rawEmpty.Architectures = nil
	if err := rawEmpty.Validate(); err == nil {
		t.Error("empty outputDir/arch: expected validation error before Normalize")
	}
}

func TestTLSModes(t *testing.T) {
	// Default is "none" and needs no CA fields.
	c := Default()
	c.Normalize()
	if c.UsesCustomTLS() {
		t.Error("default TLSMode should not use custom TLS")
	}

	// Cert modes require a CA secret name + path (both defaulted by Normalize).
	for _, mode := range []string{TLSSelfSigned, TLSCustomCA} {
		c := Default()
		c.TLSMode = mode
		c.Normalize()
		if !c.UsesCustomTLS() {
			t.Errorf("%s should use custom TLS", mode)
		}
		if err := c.Validate(); err != nil {
			t.Errorf("%s with defaults should validate: %v", mode, err)
		}
		// Blank secret name must fail (test before Normalize refills it).
		bad := Default()
		bad.TLSMode = mode
		bad.CACertSecretName = ""
		if err := bad.Validate(); err == nil {
			t.Errorf("%s with empty CACertSecretName should fail validation", mode)
		}
	}

	// Unknown mode is rejected.
	c = Default()
	c.TLSMode = "mtls"
	c.Normalize()
	if err := c.Validate(); err == nil {
		t.Error("unknown TLSMode should fail validation")
	}
}

func TestFileserverIsHTTPS(t *testing.T) {
	c := Default()
	c.FileserverURL = "https://files.corp/kre"
	if !c.FileserverIsHTTPS() {
		t.Error("https URL should report HTTPS")
	}
	c.FileserverURL = "http://files.corp/kre"
	if c.FileserverIsHTTPS() {
		t.Error("http URL should not report HTTPS")
	}
}

func TestNormalizeFillsDefaults(t *testing.T) {
	c := &Config{
		KcmVersion:    "1.4.0",
		K0sVersion:    "v1.35.4+k0s.0",
		RegistryHost:  "reg.local/",
		FileserverURL: "http://fs.local/x/",
	}
	c.Normalize()
	if c.RegistryProject == "" || c.Namespace == "" || c.OutputDir == "" {
		t.Fatal("Normalize did not fill required defaults")
	}
	if c.RegistryHost != "reg.local" {
		t.Errorf("trailing slash not trimmed: %q", c.RegistryHost)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("normalized config should validate: %v", err)
	}
}
