//go:build windows

package agent

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

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
