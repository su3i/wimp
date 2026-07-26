package agent

import (
	"os/exec"
	"strings"
)

// serviceState returns the named Windows service's current state word (RUNNING,
// STOPPED, STOP_PENDING, START_PENDING, ...), or "" if it couldn't be determined.
func serviceState(name string) string {
	out, err := exec.Command("sc.exe", "query", name).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "STATE") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return fields[len(fields)-1]
			}
		}
	}
	return ""
}

func serviceRunning(name string) bool {
	return serviceState(name) == "RUNNING"
}

func checkWindowsExporterHealthy() bool {
	return serviceRunning("windows_exporter")
}

func checkFluentBitHealthy() bool {
	return serviceRunning("fluent-bit")
}
