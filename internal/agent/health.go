package agent

import (
	"os/exec"
	"strings"
)

// serviceRunning reports whether the named Windows service is currently RUNNING.
func serviceRunning(name string) bool {
	out, err := exec.Command("sc.exe", "query", name).Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "STATE") {
			return strings.Contains(line, "RUNNING")
		}
	}
	return false
}

func checkWindowsExporterHealthy() bool {
	return serviceRunning("windows_exporter")
}

func checkFluentBitHealthy() bool {
	return serviceRunning("fluent-bit")
}
