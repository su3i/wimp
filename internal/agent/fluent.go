package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/su3i/wimp/internal/domain/protocol"
)

// applyFluentConfig reconciles the fluent-bit conf.d directory against the payload,
// writing a config file for each active pool and deleting stale ones, then restarts
// the service. fbDir is the absolute path to the fluent-bit installation directory
// as written into config.json by bootstrap.ps1.
func applyFluentConfig(fbDir string, payload protocol.FluentConfigPayload) error {
	mainConf := filepath.Join(fbDir, "fluent-bit.conf")
	confD := filepath.Join(fbDir, "conf.d")

	if err := os.MkdirAll(confD, 0755); err != nil {
		return fmt.Errorf("create conf.d: %w", err)
	}

	if err := ensureInclude(mainConf, confD); err != nil {
		return fmt.Errorf("patch fluent-bit.conf: %w", err)
	}

	activeFiles := map[string]bool{}
	for _, cfg := range payload.Configs {
		if cfg.LogPath == "" {
			continue
		}
		fname := fmt.Sprintf("wimp_pool_%d.conf", cfg.PoolID)
		content := renderPoolConfig(fbDir, cfg, payload.LokiHost, payload.LokiPort, payload.LokiTlsEnabled, payload.LokiTlsSkipVerify, payload.MachineID)
		if err := os.WriteFile(filepath.Join(confD, fname), []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", fname, err)
		}
		activeFiles[fname] = true
	}

	// Remove any pool config files no longer in the payload.
	entries, _ := os.ReadDir(confD)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "wimp_pool_") && !activeFiles[e.Name()] {
			os.Remove(filepath.Join(confD, e.Name()))
		}
	}

	return restartFluentBit()
}

// ensureInclude appends an @INCLUDE directive to the main fluent-bit config if one
// for conf.d doesn't already exist. This is a one-time operation per machine.
func ensureInclude(mainConf, confD string) error {
	data, err := os.ReadFile(mainConf)
	if err != nil {
		return err
	}
	if strings.Contains(string(data), "conf.d") {
		return nil
	}
	f, err := os.OpenFile(mainConf, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n@INCLUDE %s\\*.conf\n", confD)
	return err
}

func renderPoolConfig(fbDir string, cfg protocol.FluentAppConfig, lokiHost, lokiPort string, lokiTls bool, lokiTlsSkipVerify bool, machineID uint) string {
	tag := fmt.Sprintf("wimp.app.%d.pool.%d", cfg.ApplicationID, cfg.PoolID)
	dbDir := filepath.Join(fbDir, "db")
	os.MkdirAll(dbDir, 0755)
	dbPath := filepath.Join(dbDir, fmt.Sprintf("wimp_pool_%d.db", cfg.PoolID))

	tls := "off"
	if lokiTls {
		tls = "on"
	}
	tlsVerify := "on"
	if lokiTlsSkipVerify {
		tlsVerify = "off"
	}

	return fmt.Sprintf(`[INPUT]
    Name      tail
    Path      %s\*
    Tag       %s
    Path_Key  filename
    DB        %s

[OUTPUT]
    Name          loki
    Match         %s
    Host          %s
    Port          %s
    Tls           %s
    Tls.verify    %s
    Labels        job=wimp,application_id=%d,pool_id=%d,machine_id=%d
    Label_keys    $filename
    Workers       1
`, cfg.LogPath, tag, dbPath, tag, lokiHost, lokiPort, tls, tlsVerify, cfg.ApplicationID, cfg.PoolID, machineID)
}

func restartFluentBit() error {
	// Ignore stop failure - service may already be stopped.
	exec.Command("sc.exe", "stop", "fluent-bit").Run()
	if out, err := exec.Command("sc.exe", "start", "fluent-bit").CombinedOutput(); err != nil {
		return fmt.Errorf("start fluent-bit: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
