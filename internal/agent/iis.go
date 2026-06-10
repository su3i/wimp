package agent

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/su3i/wimp/internal/domain/protocol"
)

func appcmdPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "inetsrv", "appcmd.exe")
}

func iisAvailable() bool {
	_, err := os.Stat(appcmdPath())
	return err == nil
}

// ── XML structs for parsing appcmd /xml output ────────────────────────────────

type appcmdOutput struct {
	AppPools []xmlAppPool `xml:"APPPOOL"`
	Sites    []xmlSite    `xml:"SITE"`
}

type xmlAppPool struct {
	Name           string        `xml:"APPPOOL.NAME,attr"`
	PipelineMode   string        `xml:"PipelineMode,attr"`
	RuntimeVersion string        `xml:"RuntimeVersion,attr"`
	State          string        `xml:"state,attr"`
	Extra          xmlAppPoolEx  `xml:"appPoolEx"`
}

type xmlAppPoolEx struct {
	StartMode    string `xml:"startMode,attr"`
	IdentityType string `xml:"identityType,attr"`
}

type xmlSite struct {
	Name string      `xml:"SITE.NAME,attr"`
	State string     `xml:"state,attr"`
	Body xmlSiteBody `xml:"site"`
}

type xmlSiteBody struct {
	Application xmlApplication `xml:"application"`
	Bindings    []xmlBinding   `xml:"bindings>binding"`
}

type xmlApplication struct {
	Pool string    `xml:"applicationPool,attr"`
	VDir xmlVDir   `xml:"virtualDirectory"`
}

type xmlVDir struct {
	PhysicalPath string `xml:"physicalPath,attr"`
}

type xmlBinding struct {
	Protocol           string `xml:"protocol,attr"`
	BindingInformation string `xml:"bindingInformation,attr"`
}

// ── Discovery ─────────────────────────────────────────────────────────────────

func discoverAppPools() []protocol.AppPoolInfo {
	if !iisAvailable() {
		return nil
	}
	out, err := exec.Command(appcmdPath(), "list", "apppool", "/xml").Output()
	if err != nil {
		return nil
	}
	var result appcmdOutput
	if err := xml.Unmarshal(out, &result); err != nil {
		return nil
	}
	pools := make([]protocol.AppPoolInfo, 0, len(result.AppPools))
	for _, p := range result.AppPools {
		pools = append(pools, protocol.AppPoolInfo{
			Name:           p.Name,
			State:          p.State,
			RuntimeVersion: p.RuntimeVersion,
			PipelineMode:   p.PipelineMode,
			StartMode:      p.Extra.StartMode,
			IdentityType:   p.Extra.IdentityType,
		})
	}
	return pools
}

func discoverSites() []protocol.SiteInfo {
	if !iisAvailable() {
		return nil
	}
	out, err := exec.Command(appcmdPath(), "list", "site", "/xml").Output()
	if err != nil {
		return nil
	}
	var result appcmdOutput
	if err := xml.Unmarshal(out, &result); err != nil {
		return nil
	}
	sites := make([]protocol.SiteInfo, 0, len(result.Sites))
	for _, s := range result.Sites {
		bindings := make([]protocol.BindingInfo, 0, len(s.Body.Bindings))
		for _, b := range s.Body.Bindings {
			bindings = append(bindings, parseBinding(b.Protocol, b.BindingInformation))
		}
		sites = append(sites, protocol.SiteInfo{
			Name:         s.Name,
			State:        s.State,
			PhysicalPath: s.Body.Application.VDir.PhysicalPath,
			AppPoolName:  s.Body.Application.Pool,
			Bindings:     bindings,
		})
	}
	return sites
}

// ── Heartbeat helpers ─────────────────────────────────────────────────────────

func runningAppPools() []string {
	if !iisAvailable() {
		return nil
	}
	out, err := exec.Command(appcmdPath(), "list", "apppool", "/state:Started", "/xml").Output()
	if err != nil {
		return nil
	}
	var result appcmdOutput
	if err := xml.Unmarshal(out, &result); err != nil {
		return nil
	}
	names := make([]string, 0, len(result.AppPools))
	for _, p := range result.AppPools {
		names = append(names, p.Name)
	}
	return names
}

func runningSites() []string {
	if !iisAvailable() {
		return nil
	}
	out, err := exec.Command(appcmdPath(), "list", "site", "/state:Started", "/xml").Output()
	if err != nil {
		return nil
	}
	var result appcmdOutput
	if err := xml.Unmarshal(out, &result); err != nil {
		return nil
	}
	names := make([]string, 0, len(result.Sites))
	for _, s := range result.Sites {
		names = append(names, s.Name)
	}
	return names
}

// ── Command execution ─────────────────────────────────────────────────────────

func executeCommand(action, targetType, target string) (string, error) {
	if !iisAvailable() {
		return "", fmt.Errorf("IIS not available on this machine")
	}

	cmd := appcmdPath()

	switch targetType {
	case "app_pool":
		nameArg := fmt.Sprintf("/apppool.name:%s", target)
		switch action {
		case "start":
			return run(cmd, "start", "apppool", nameArg)
		case "stop":
			return run(cmd, "stop", "apppool", nameArg)
		case "recycle":
			return run(cmd, "recycle", "apppool", nameArg)
		case "restart":
			if _, err := run(cmd, "stop", "apppool", nameArg); err != nil {
				return "", fmt.Errorf("stop failed: %w", err)
			}
			return run(cmd, "start", "apppool", nameArg)
		default:
			return "", fmt.Errorf("unknown action: %s", action)
		}
	default:
		return "", fmt.Errorf("unknown target type: %s", targetType)
	}
}

func run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("appcmd: %w", err)
	}
	return output, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// parseBinding parses an IIS binding information string (ip:port:hostname).
func parseBinding(proto, info string) protocol.BindingInfo {
	parts := strings.SplitN(info, ":", 3)
	b := protocol.BindingInfo{Protocol: proto}
	if len(parts) >= 1 {
		b.IP = parts[0]
	}
	if len(parts) >= 2 {
		b.Port = parts[1]
	}
	if len(parts) >= 3 {
		b.Hostname = parts[2]
	}
	return b
}
