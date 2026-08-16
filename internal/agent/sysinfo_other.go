//go:build !windows

package agent

func windowsVersion() string { return "" }

func machineUID() string { return "" }
