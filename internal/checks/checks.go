// Package checks actively probes the deployment targets from the machine running
// airgapdeploy — the machine whose access actually matters. Each check returns a
// structured result with a concrete reason so the UI can explain failures and
// highlight the responsible config field.
package checks

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/michaeltillman/airgapdeploy/internal/config"
)

// Status values for a Result.
const (
	Pass = "pass" // target reachable / verified
	Fail = "fail" // hard failure that will break the install
	Warn = "warn" // not reachable yet or optional; may be expected mid-flow
	Skip = "skip" // could not run the check (e.g. tool missing)
)

// Result is one access/connectivity check outcome.
type Result struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Field  string `json:"field"`  // config field to highlight, "" if none
	Status string `json:"status"` // pass|fail|warn|skip
	Detail string `json:"detail"` // human-readable reason
}

// Run executes all target-access checks for cfg concurrently and returns their
// results in a stable order.
func Run(cfg *config.Config) []Result {
	jobs := []func(*config.Config) Result{}
	jobs = append(jobs, checkOutputDir)
	if cfg.UsesCustomTLS() {
		jobs = append(jobs, checkCACert)
	}
	if cfg.UsesACME() {
		jobs = append(jobs, checkACME)
		if cfg.ACMEIngressClass != "" {
			jobs = append(jobs, checkIngressClass)
		}
	}
	jobs = append(jobs,
		checkRegistry,
		checkNginxImage,
		checkBusyboxImage,
		checkVersionAvailable,
		checkK0sBinaries,
		checkFileserver,
		checkStorageClass,
		checkCluster,
	)

	out := make([]Result, len(jobs))
	var wg sync.WaitGroup
	for i, fn := range jobs {
		wg.Add(1)
		go func(i int, fn func(*config.Config) Result) {
			defer wg.Done()
			out[i] = fn(cfg)
		}(i, fn)
	}
	wg.Wait()
	return out
}

func hostPort(registryHost string, https bool) (host, hostport string) {
	host = registryHost
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	if strings.Contains(host, ":") {
		return strings.SplitN(host, ":", 2)[0], host
	}
	if https {
		return host, host + ":443"
	}
	return host, host + ":80"
}

// httpClient builds a client that trusts caPath (if set), or skips verification
// when insecure is true (TLS mode "none").
func httpClient(caPath string, insecure bool) (*http.Client, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: insecure} //nolint:gosec // insecure only for TLS mode "none"
	if caPath != "" {
		pem, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert %s: %w", caPath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s is not a valid PEM certificate", caPath)
		}
		tlsCfg = &tls.Config{RootCAs: pool}
	}
	return &http.Client{
		Timeout:   7 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}, nil
}

func checkCACert(cfg *config.Config) Result {
	r := Result{ID: "cacert", Label: "CA certificate file", Field: "caCertPath"}
	info, err := os.Stat(cfg.CACertPath)
	if err != nil {
		r.Status, r.Detail = Fail, fmt.Sprintf("cannot read %s: %v", cfg.CACertPath, err)
		return r
	}
	if info.IsDir() {
		r.Status, r.Detail = Fail, fmt.Sprintf("%s is a directory, not a certificate file", cfg.CACertPath)
		return r
	}
	pem, err := os.ReadFile(cfg.CACertPath)
	if err != nil {
		r.Status, r.Detail = Fail, fmt.Sprintf("cannot read %s: %v", cfg.CACertPath, err)
		return r
	}
	if !x509.NewCertPool().AppendCertsFromPEM(pem) {
		r.Status, r.Detail = Fail, fmt.Sprintf("%s does not contain a valid PEM certificate", cfg.CACertPath)
		return r
	}
	r.Status, r.Detail = Pass, fmt.Sprintf("valid PEM certificate at %s", cfg.CACertPath)
	return r
}

func checkRegistry(cfg *config.Config) Result {
	r := Result{ID: "registry", Label: "Registry reachable & trusted", Field: "registryHost"}
	host, hp := hostPort(cfg.RegistryHost, true)

	// 1. TCP.
	conn, err := net.DialTimeout("tcp", hp, 5*time.Second)
	if err != nil {
		r.Status, r.Detail = Fail, fmt.Sprintf("cannot connect to %s: %v (DNS, firewall, or registry down)", hp, err)
		return r
	}
	_ = conn.Close()

	// 2. TLS + /v2/. For cert modes, verify against the CA (no insecure skip) so
	// an x509 error is a real trust failure. For "none", skip verification.
	caPath := ""
	insecure := true
	if cfg.UsesCustomTLS() {
		caPath, insecure = cfg.CACertPath, false
	} else if cfg.UsesACME() {
		insecure = false // ACME certs are publicly trusted; verify with system roots
	}
	client, err := httpClient(caPath, insecure)
	if err != nil {
		r.Status, r.Detail = Fail, err.Error()
		return r
	}
	resp, err := client.Get("https://" + hp + "/v2/")
	if err != nil {
		if strings.Contains(err.Error(), "x509") || strings.Contains(err.Error(), "certificate") {
			r.Status, r.Detail = Fail, fmt.Sprintf("TLS trust failed: %v — the registry certificate is not signed by the provided CA", err)
			return r
		}
		r.Status, r.Detail = Fail, fmt.Sprintf("registry request failed: %v", err)
		return r
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
		if insecure {
			r.Status, r.Detail = Pass, "reachable at "+host+" (TLS not verified — mode 'none')"
		} else {
			r.Status, r.Detail = Pass, "reachable and TLS verified against the CA"
		}
	case 401:
		r.Status, r.Detail = Fail, "registry requires authentication (this build targets a non-authenticated registry)"
	default:
		r.Status, r.Detail = Warn, fmt.Sprintf("registry /v2/ returned HTTP %d", resp.StatusCode)
	}
	return r
}

// splitImageTag splits "name:tag" into its parts (tag empty if none).
func splitImageTag(ref string) (name, tag string) {
	if i := strings.LastIndexByte(ref, ':'); i >= 0 && !strings.Contains(ref[i:], "/") {
		return ref[:i], ref[i+1:]
	}
	return ref, ""
}

func registryClient(cfg *config.Config) (*http.Client, error) {
	caPath, insecure := "", true
	if cfg.UsesCustomTLS() {
		caPath, insecure = cfg.CACertPath, false
	} else if cfg.UsesACME() {
		insecure = false
	}
	return httpClient(caPath, insecure)
}

// checkImageTag verifies that a specific image tag exists in the registry.
func checkImageTag(cfg *config.Config, id, label, field, repo, tag string) Result {
	r := Result{ID: id, Label: label, Field: field}
	client, err := registryClient(cfg)
	if err != nil {
		r.Status, r.Detail = Skip, err.Error()
		return r
	}
	_, hp := hostPort(cfg.RegistryHost, true)
	resp, err := client.Get("https://" + hp + "/v2/" + repo + "/tags/list")
	if err != nil {
		r.Status, r.Detail = Warn, fmt.Sprintf("could not query %s: %v", repo, err)
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		r.Status, r.Detail = Warn, fmt.Sprintf("%s not found (HTTP %d) — run 01-prepare.sh to push images", repo, resp.StatusCode)
		return r
	}
	var body struct {
		Tags []string `json:"tags"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	for _, t := range body.Tags {
		if t == tag {
			r.Status, r.Detail = Pass, fmt.Sprintf("%s:%s present in the registry", repo, tag)
			return r
		}
	}
	r.Status, r.Detail = Warn, fmt.Sprintf("%s exists but tag %q is missing — check the image version", repo, tag)
	return r
}

func checkNginxImage(cfg *config.Config) Result {
	name, tag := splitImageTag(cfg.NginxImage) // served from the registry root
	return checkImageTag(cfg, "img-nginx", "nginx image present", "nginxImage", name, tag)
}

func checkBusyboxImage(cfg *config.Config) Result {
	name, tag := splitImageTag(cfg.BusyboxImage) // lives under the project
	return checkImageTag(cfg, "img-busybox", "busybox image present", "busyboxImage", cfg.RegistryProject+"/"+name, tag)
}

func checkK0sBinaries(cfg *config.Config) Result {
	r := Result{ID: "k0s", Label: "k0s binaries available", Field: "k0sVersion"}
	client := &http.Client{Timeout: 7 * time.Second}
	var missing, unreachable []string
	for _, arch := range cfg.Architectures {
		url := fmt.Sprintf("%s/k0s/%s", cfg.BundleBaseURL, cfg.K0sBinaryName(arch))
		req, _ := http.NewRequest(http.MethodHead, url, nil)
		resp, err := client.Do(req)
		if err != nil {
			unreachable = append(unreachable, arch)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			missing = append(missing, arch)
		}
	}
	switch {
	case len(missing) > 0:
		r.Status, r.Detail = Fail, fmt.Sprintf("no k0s %s binary for arch(s) %s (check the k0s version)", cfg.K0sVersion, strings.Join(missing, ", "))
	case len(unreachable) > 0:
		r.Status, r.Detail = Warn, fmt.Sprintf("could not reach %s to verify k0s binaries (offline?)", cfg.BundleBaseURL)
	default:
		r.Status, r.Detail = Pass, fmt.Sprintf("k0s %s available for %s", cfg.K0sVersion, strings.Join(cfg.Architectures, ", "))
	}
	return r
}

func checkOutputDir(cfg *config.Config) Result {
	r := Result{ID: "outputdir", Label: "Output directory writable", Field: "outputDir"}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		r.Status, r.Detail = Fail, fmt.Sprintf("cannot create %s: %v", cfg.OutputDir, err)
		return r
	}
	f, err := os.CreateTemp(cfg.OutputDir, ".airgapdeploy-write-*")
	if err != nil {
		r.Status, r.Detail = Fail, fmt.Sprintf("%s is not writable: %v", cfg.OutputDir, err)
		return r
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	r.Status, r.Detail = Pass, "artifacts can be written to "+cfg.OutputDir
	return r
}

func checkStorageClass(cfg *config.Config) Result {
	r := Result{ID: "storageclass", Label: "StorageClass available", Field: "storageClass"}
	if !kubectlAvailable() {
		r.Status, r.Detail = Skip, "kubectl not found; cannot verify the StorageClass"
		return r
	}
	if cfg.StorageClass != "" {
		out, err := kubectlRun("get", "storageclass", cfg.StorageClass)
		switch {
		case err == nil:
			r.Status, r.Detail = Pass, fmt.Sprintf("StorageClass %q exists", cfg.StorageClass)
		case kubectlNotFound(out):
			r.Status, r.Detail = Fail, fmt.Sprintf("StorageClass %q does not exist in the cluster", cfg.StorageClass)
		default:
			r.Status, r.Detail = Warn, fmt.Sprintf("could not verify StorageClass %q (cluster unreachable?): %s", cfg.StorageClass, firstLine(out))
		}
		return r
	}
	// No class pinned: confirm the cluster has at least one (default) class.
	out, err := kubectlRun("get", "storageclass", "-o", "name")
	if err != nil {
		r.Status, r.Detail = Warn, "could not list StorageClasses: "+firstLine(out)
		return r
	}
	if strings.TrimSpace(out) == "" {
		r.Status, r.Detail = Warn, "no StorageClass in the cluster — the fileserver PVC will stay Pending"
	} else {
		r.Status, r.Detail = Pass, "cluster has a StorageClass (using the default)"
	}
	return r
}

func checkIngressClass(cfg *config.Config) Result {
	r := Result{ID: "ingressclass", Label: "Ingress class available", Field: "acmeIngressClass"}
	if !kubectlAvailable() {
		r.Status, r.Detail = Skip, "kubectl not found; cannot verify the ingress class"
		return r
	}
	out, err := kubectlRun("get", "ingressclass", cfg.ACMEIngressClass)
	switch {
	case err == nil:
		r.Status, r.Detail = Pass, fmt.Sprintf("ingressclass %q exists", cfg.ACMEIngressClass)
	case kubectlNotFound(out):
		r.Status, r.Detail = Fail, fmt.Sprintf("ingressclass %q does not exist (needed for the ACME HTTP-01 solver)", cfg.ACMEIngressClass)
	default:
		r.Status, r.Detail = Warn, fmt.Sprintf("could not verify ingressclass %q (cluster unreachable?): %s", cfg.ACMEIngressClass, firstLine(out))
	}
	return r
}

func kubectlNotFound(out string) bool {
	l := strings.ToLower(out)
	return strings.Contains(l, "notfound") || strings.Contains(l, "not found")
}

func kubectlAvailable() bool {
	_, err := exec.LookPath("kubectl")
	return err == nil
}

func kubectlRun(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	full := append([]string{}, args...)
	full = append(full, "--request-timeout=6s")
	b, err := exec.CommandContext(ctx, "kubectl", full...).CombinedOutput()
	return string(b), err
}

func checkVersionAvailable(cfg *config.Config) Result {
	r := Result{ID: "version", Label: "Version bundle available", Field: "kcmVersion"}
	url := fmt.Sprintf("%s/%s/%s", cfg.BundleBaseURL, cfg.KcmVersion, cfg.BundleName())
	client := &http.Client{Timeout: 7 * time.Second}
	req, _ := http.NewRequest(http.MethodHead, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		r.Status, r.Detail = Warn, fmt.Sprintf("could not reach %s (offline?) — %v", cfg.BundleBaseURL, err)
		return r
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
		r.Status, r.Detail = Pass, fmt.Sprintf("bundle for %s exists upstream", cfg.KcmVersion)
	case 403, 404:
		r.Status, r.Detail = Fail, fmt.Sprintf("no bundle for version %q at %s (check the version)", cfg.KcmVersion, url)
	default:
		r.Status, r.Detail = Warn, fmt.Sprintf("unexpected HTTP %d checking the version bundle", resp.StatusCode)
	}
	return r
}

func checkFileserver(cfg *config.Config) Result {
	r := Result{ID: "fileserver", Label: "k0s fileserver reachable", Field: "fileserverURL"}
	caPath, insecure := "", false
	if cfg.FileserverIsHTTPS() {
		if cfg.UsesCustomTLS() {
			caPath = cfg.CACertPath
		} else if cfg.UsesACME() {
			// publicly trusted; verify with system roots
		} else {
			insecure = true
		}
	}
	client, err := httpClient(caPath, insecure)
	if err != nil {
		r.Status, r.Detail = Skip, err.Error()
		return r
	}
	url := cfg.FileserverURL + "/" + cfg.PrimaryK0sBinary()
	req, _ := http.NewRequest(http.MethodHead, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		r.Status, r.Detail = Warn, fmt.Sprintf("not reachable yet: %v (deploy the fileserver in step 2)", err)
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		r.Status, r.Detail = Pass, "serving the k0s binary"
	} else {
		r.Status, r.Detail = Warn, fmt.Sprintf("HTTP %d for the k0s binary (not uploaded/deployed yet?)", resp.StatusCode)
	}
	return r
}

func checkACME(cfg *config.Config) Result {
	r := Result{ID: "acme", Label: "ACME server reachable", Field: "acmeServer"}
	client := &http.Client{Timeout: 7 * time.Second}
	resp, err := client.Get(cfg.ACMEServer)
	if err != nil {
		r.Status, r.Detail = Warn, fmt.Sprintf("could not reach %s: %v (cert-manager needs this to issue certs)", cfg.ACMEServer, err)
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		r.Status, r.Detail = Pass, "ACME directory reachable at "+cfg.ACMEServer
	} else {
		r.Status, r.Detail = Warn, fmt.Sprintf("ACME directory returned HTTP %d", resp.StatusCode)
	}
	return r
}

func checkCluster(cfg *config.Config) Result {
	r := Result{ID: "cluster", Label: "Kubernetes cluster access", Field: ""}
	if _, err := exec.LookPath("kubectl"); err != nil {
		r.Status, r.Detail = Skip, "kubectl not found on this machine (needed for the cluster-side steps)"
		return r
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "get", "--raw=/readyz", "--request-timeout=6s")
	b, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(b))
		if detail == "" {
			detail = err.Error()
		}
		r.Status, r.Detail = Fail, "cannot reach the cluster: "+firstLine(detail)
		return r
	}
	r.Status, r.Detail = Pass, "cluster API reachable ("+strings.TrimSpace(string(b))+")"
	return r
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
