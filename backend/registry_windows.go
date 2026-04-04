//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// writeServiceEnv writes per-service environment variables to the Windows
// registry under HKLM\SYSTEM\CurrentControlSet\Services\<name>\Environment
// as a REG_MULTI_SZ value. The SCM reads this block and injects the variables
// into the service process environment at startup.
func writeServiceEnv(serviceName string, env []string) error {
	keyPath := `SYSTEM\CurrentControlSet\Services\` + serviceName
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key %s: %w", keyPath, err)
	}
	defer k.Close()
	return k.SetStringsValue("Environment", env)
}
