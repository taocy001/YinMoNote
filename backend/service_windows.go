//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const winsvcName = "YinMoNote"

// runAsWindowsService checks whether the process was launched by the Windows
// Service Control Manager. If so, it registers the service handler, blocks
// until the SCM sends a stop signal, and returns true. Returns false when
// running as a normal console process.
func runAsWindowsService(lib *NoteLibrary, port string) bool {
	isSvc, err := svc.IsWindowsService()
	if err != nil || !isSvc {
		return false
	}
	if err := svc.Run(winsvcName, &yinmoService{lib: lib, port: port}); err != nil {
		fmt.Fprintf(os.Stderr, "YinMo: service run failed: %v\n", err)
	}
	return true
}

// yinmoService implements svc.Handler for the Windows Service Control Manager.
type yinmoService struct {
	lib  *NoteLibrary
	port string
}

// Execute is called by the SCM when the service starts. It launches the
// application in a goroutine, reports Running, then waits for a stop signal.
// Notes are written atomically to disk before any git commit, so at most the
// last auto-commit cycle (≤5 min) is skipped on forced shutdown — acceptable
// for a single-user personal notes server.
func (s *yinmoService) Execute(_ []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	go runApplication(s.lib, s.port)
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for c := range r {
		switch c.Cmd {
		case svc.Stop, svc.Shutdown:
			status <- svc.Status{State: svc.StopPending}
			os.Exit(0)
		}
	}
	return false, 0
}

// InstallWindowsService registers yinmonote.exe as an auto-start Windows
// Service. Requires administrator privileges. env is a list of "KEY=VALUE"
// strings stored in the service's registry environment block, which the SCM
// injects into the process environment at startup.
func InstallWindowsService(exePath string, env []string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	// Idempotent: remove stale service before re-creating.
	if existing, err := m.OpenService(winsvcName); err == nil {
		_, _ = existing.Control(svc.Stop)
		_ = existing.Delete()
		existing.Close()
	}

	s, err := m.CreateService(winsvcName, exePath, mgr.Config{
		DisplayName: "YinMoNote Note Server",
		Description: "Self-hosted personal note library (yinmonote.dev)",
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	// Per-service environment variables are stored in the registry under
	// HKLM\SYSTEM\CurrentControlSet\Services\<Name>\Environment (REG_MULTI_SZ).
	// The Install.ps1 script handles this via Set-ItemProperty; the Go path
	// is provided for programmatic installs.
	if len(env) > 0 {
		if err := writeServiceEnv(winsvcName, env); err != nil {
			return fmt.Errorf("set service environment: %w", err)
		}
	}
	return nil
}

// RemoveWindowsService stops and deletes the YinMoNote Windows Service.
// Requires administrator privileges. Returns nil if the service does not exist.
func RemoveWindowsService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(winsvcName)
	if err != nil {
		return nil // already absent
	}
	defer s.Close()
	_, _ = s.Control(svc.Stop)
	return s.Delete()
}
