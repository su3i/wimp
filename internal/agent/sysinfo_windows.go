//go:build windows

package agent

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// machineUID returns the Windows MachineGuid - a per-installation identifier written at
// OS install time and stable across reboots, hostname changes, and re-running the wimp
// bootstrap. The control plane uses it to recognize that a host bootstrapped into a new
// project is the same physical box as an existing one. Returns "" if the key can't be
// read, in which case the control plane falls back to matching on hostname.
func machineUID() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer k.Close()

	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return ""
	}
	return guid
}

func windowsVersion() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()

	productName, _, err := k.GetStringValue("ProductName")
	if err != nil {
		return ""
	}

	build, _, _ := k.GetStringValue("CurrentBuild")
	ubr, _, _ := k.GetIntegerValue("UBR")

	if build != "" {
		return fmt.Sprintf("%s (Build %s.%d)", productName, build, ubr)
	}
	return productName
}
