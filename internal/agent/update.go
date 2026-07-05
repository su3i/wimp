package agent

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const updateApplyDelay = 10 * time.Second

// updateAgent downloads a new agent build and schedules a one-shot Windows Scheduled
// Task to stop the service, swap the binary, and start it again. The running process
// never touches its own live executable directly - the swap only happens once the
// service has actually stopped, decoupled from this process's lifetime rather than
// timed against it.
func updateAgent(downloadURL string) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not resolve executable path: %w", err)
	}

	stagedPath := filepath.Join(filepath.Dir(exePath), "agent.update.exe")

	if err := downloadFile(downloadURL, stagedPath); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	if err := scheduleSwapAndRestart(stagedPath, exePath); err != nil {
		os.Remove(stagedPath) //nolint:errcheck
		return "", fmt.Errorf("failed to schedule update: %w", err)
	}

	return fmt.Sprintf("update downloaded; applying in ~%s via scheduled task", updateApplyDelay), nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	if n < 1024 {
		return fmt.Errorf("downloaded file suspiciously small (%d bytes)", n)
	}
	return nil
}

// scheduleSwapAndRestart creates a one-shot Windows Scheduled Task a few seconds in
// the future that stops the service, moves the downloaded binary into place, starts the
// service back up, and deletes the task itself. Using Task Scheduler rather than a
// detached helper process means the swap happens deterministically once the service
// has actually stopped, with no dependency on this process's own lifetime.
//
// The actual stop/move/start sequence lives in a small batch script rather than being
// crammed into schtasks' /TR argument as a single quoted string: nesting quotes inside
// a `cmd /c "..."` value only works when a shell is the one typing/escaping it, and
// doesn't survive being handed through exec.Command and then re-invoked later by Task
// Scheduler itself - that mismatch let `/Create` succeed while the stored task's command
// line was silently malformed, so it fired but did nothing.
func scheduleSwapAndRestart(stagedPath, exePath string) error {
	const taskName = "WimpAgentUpdate"

	scriptPath := filepath.Join(filepath.Dir(exePath), "wimp_update.bat")
	script := fmt.Sprintf(
		"net stop %s\r\nmove /Y \"%s\" \"%s\"\r\nnet start %s\r\nschtasks /Delete /TN %s /F\r\ndel \"%%~f0\"\r\n",
		ServiceName, stagedPath, exePath, ServiceName, taskName,
	)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("write update script: %w", err)
	}

	startTime := time.Now().Add(updateApplyDelay).Format("15:04:05")

	cmd := exec.Command("schtasks",
		"/Create", "/SC", "ONCE",
		"/ST", startTime,
		"/TN", taskName,
		"/TR", `"`+scriptPath+`"`,
		"/RU", "SYSTEM",
		"/F",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(scriptPath) //nolint:errcheck
		return fmt.Errorf("schtasks: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
