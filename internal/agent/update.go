package agent

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// updateAgent downloads a new agent build and fires a Windows Scheduled Task
// immediately to stop the service, swap the binary, and start it again. The running
// process never touches its own live executable directly - the swap only happens once
// the service has actually stopped, decoupled from this process's lifetime rather than
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

	return "update downloaded; applying in a few seconds via scheduled task", nil
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

// scheduleSwapAndRestart creates a Windows Scheduled Task that stops the service,
// moves the downloaded binary into place, starts the service back up, and deletes the
// task itself - then fires it immediately via `schtasks /Run` rather than scheduling it
// for a computed future time. `/Run` starts the task asynchronously (it doesn't wait
// for completion) with no wall-clock parsing involved at all, which sidesteps a real
// failure mode we hit with `/Create ... /ST <time>`: Task Scheduler's time parsing can
// silently schedule the task for the wrong moment (or one that's already considered
// past), so the task gets created successfully but never visibly fires. Using Task
// Scheduler at all (rather than a detached helper process) still means the swap happens
// deterministically once the service has actually stopped, decoupled from this
// process's own lifetime.
//
// The actual stop/move/start sequence lives in a small batch script rather than being
// crammed into schtasks' /TR argument as a single quoted string: nesting quotes inside
// a `cmd /c "..."` value only works when a shell is the one typing/escaping it, and
// doesn't survive being handed through exec.Command and then re-invoked later by Task
// Scheduler itself - that mismatch let `/Create` succeed while the stored task's command
// line was silently malformed, so it fired but did nothing. The script opens with a
// `ping`-based delay (not `timeout`, which can fail outright with no console attached,
// exactly the context a SYSTEM scheduled task runs in) so our own CommandResult reply
// has time to reach the control plane before the service goes down.
func scheduleSwapAndRestart(stagedPath, exePath string) error {
	const taskName = "WimpAgentUpdate"

	scriptPath := filepath.Join(filepath.Dir(exePath), "wimp_update.bat")
	script := fmt.Sprintf(
		"ping -n 6 127.0.0.1 >nul\r\nnet stop %s\r\nmove /Y \"%s\" \"%s\"\r\nnet start %s\r\nschtasks /Delete /TN %s /F\r\ndel \"%%~f0\"\r\n",
		ServiceName, stagedPath, exePath, ServiceName, taskName,
	)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("write update script: %w", err)
	}

	createOut, err := exec.Command("schtasks",
		"/Create", "/SC", "ONCE",
		"/ST", "00:00",
		"/TN", taskName,
		"/TR", `"`+scriptPath+`"`,
		"/RU", "SYSTEM",
		"/F",
	).CombinedOutput()
	if err != nil {
		os.Remove(scriptPath) //nolint:errcheck
		return fmt.Errorf("schtasks create: %w (%s)", err, strings.TrimSpace(string(createOut)))
	}

	runOut, err := exec.Command("schtasks", "/Run", "/TN", taskName).CombinedOutput()
	if err != nil {
		os.Remove(scriptPath) //nolint:errcheck
		return fmt.Errorf("schtasks run: %w (%s)", err, strings.TrimSpace(string(runOut)))
	}
	return nil
}
