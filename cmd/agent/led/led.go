// Package led provides optional drive-bay LED identification via ledctl.
// If ledctl is not installed on the host, all operations gracefully degrade.
package led

import (
	"fmt"
	"os/exec"
	"strings"
)

// Controller wraps the ledctl binary for drive-bay LED identification.
// A nil Controller is safe to use — all methods return ErrUnavailable.
type Controller struct {
	path string
}

// ErrUnavailable is returned when ledctl is not installed on the host.
var ErrUnavailable = fmt.Errorf("ledctl not available on this host")

// Detect probes for ledctl on the system PATH. Returns nil if not found
// (the caller should treat a nil *Controller as "feature disabled").
func Detect() *Controller {
	path, err := exec.LookPath("ledctl")
	if err != nil {
		return nil
	}
	return &Controller{path: path}
}

// Available reports whether LED control is possible on this host.
func (c *Controller) Available() bool {
	return c != nil && c.path != ""
}

// Identify toggles the drive-bay LED for a device.
// mode is "locate" (LED on) or "off" (LED back to normal).
func (c *Controller) Identify(device, mode string) (string, error) {
	if !c.Available() {
		return "", ErrUnavailable
	}

	verb := "locate"
	if mode == "off" {
		verb = "normal"
	}

	path := device
	if !strings.HasPrefix(path, "/") {
		path = "/dev/" + path
	}

	arg := verb + "=" + path
	cmd := exec.Command(c.path, arg) // #nosec G204 -- path is from LookPath at startup
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("ledctl %s: %w: %s", arg, err, output)
	}
	return output, nil
}
