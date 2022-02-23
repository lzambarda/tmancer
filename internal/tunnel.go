// Package internal contains everything you do not want to expose.
package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

var signalRegex = regexp.MustCompile(`signal: ([a-z ]+)$`)

// K8sInfo contains all information required to use a kubectl port forward command.
type K8sInfo struct {
	Namespace string `json:"namespace"`
	Service   string `json:"service"`
	Port      int    `json:"port"`
}

// TunnelConfig is just what its name suggests. There are two supported configs:
// "k8s" and "custom".
type TunnelConfig struct {
	Name      string   `json:"name"`
	K8s       *K8sInfo `json:"k8s"`
	Custom    string   `json:"custom"`
	LocalPort int      `json:"local_port"`
}

// GetType returns the config type being used. See the description of
// TunnelConfig to know which ones are available.
func (c *TunnelConfig) GetType() string {
	if c.K8s != nil {
		return "k8s"
	}
	if c.Custom != "" {
		return "custom"
	}
	return "N/A"
}

//nolint:gosec // I'm happy for now.
func (c *TunnelConfig) getCommand(ctx context.Context) (*exec.Cmd, error) {
	if c.K8s != nil {
		return exec.CommandContext(ctx, "kubectl", "port-forward", "-n", c.K8s.Namespace, c.K8s.Service, fmt.Sprintf("%d:%d", c.LocalPort, c.K8s.Port)), nil
	}
	if c.Custom != "" {
		parts := strings.Split(c.Custom, " ")
		return exec.CommandContext(ctx, parts[0], parts[1:]...), nil
	}
	return nil, errors.New("config is missing command information")
}

// Tunnel is our mighty tunnel structure. Do not initialise this structure
// directly but use NewTunnel instead.
type Tunnel struct {
	cmd         *exec.Cmd
	err         error
	startedAt   time.Time
	config      TunnelConfig
	status      Status
	startedFlag int32
}

// NewTunnel instantiates a usable Tunnel object.
func NewTunnel(config TunnelConfig) *Tunnel {
	return &Tunnel{
		status:      Close,
		config:      config,
		startedFlag: 0,
	}
}

// GetPid returns the pid of the subprocess used by this tunnel. Returns 0 if
// the process is not available.
func (t *Tunnel) GetPid() int {
	if t.cmd != nil && t.cmd.Process != nil {
		return t.cmd.Process.Pid
	}
	return 0
}

// GetStatus returns the current tunnel status.
func (t *Tunnel) GetStatus() Status {
	return t.status
}

// GetError returns the message of the last occurred error, empty string
// otherwise.
func (t *Tunnel) GetError() string {
	if t.err == nil {
		return ""
	}
	return t.err.Error()
}

// GetAge returns a duration value expressing how long this tunnel has been in
// the "Open" status. The valid flag tells whether the age is valid or not.
// It is resets when the tunnel changes status.
func (t *Tunnel) GetAge() (age time.Duration, valid bool) {
	if t.status == Open {
		return time.Since(t.startedAt).Round(time.Second), true
	}
	return time.Duration(0), false
}

func (t *Tunnel) kill() {
	if t.cmd == nil || t.cmd.Process == nil {
		return
	}
	err := t.cmd.Process.Kill()
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		fmt.Printf("Error while killing %s: %v\n", t.config.Name, err)
	}
}

//nolint:gosec // I'm happy for now.
func isPortBusy(ctx context.Context, port int) bool {
	cmd := exec.CommandContext(ctx, "lsof", "-i", fmt.Sprintf(":%d", port))
	cmd.Run() // nolint:errcheck // lsof returns error if nothing is found.
	// If the process state is nil it means that the command could not be
	// completed.
	// In this case we simply treat this as busy (for now, not the best).
	if cmd.ProcessState == nil {
		return true
	}
	return cmd.ProcessState.Success()
}

// Start the tunnel with a given context, lock and wait group. Better to run
// this in a separate goroutine.
func (t *Tunnel) Start(ctx context.Context, m sync.Locker) {
	// Make sure that this hasn't been started twice.
	if atomic.SwapInt32(&t.startedFlag, 1) != 0 {
		return
	}
	ch := make(chan error)
	var err error
	for {
		m.Lock()
		select {
		case <-ctx.Done():
			t.kill()
			m.Unlock()
			return
		case err = <-ch:
			switch {
			case err == nil:
				// Tunnel closed with no error
				t.status = Cooper
			case signalRegex.MatchString(err.Error()):
				t.status = Signal
				t.err = errors.New(signalRegex.FindString(err.Error()))
			default:
				t.status = Error
				t.err = err
			}
		default:
		}
		switch t.status {
		// All statuses leading to (re)opening the tunnel.
		case Close, Reopening, Cooper, PortBusy:
			// First check if the port is busy
			if isPortBusy(ctx, t.config.LocalPort) {
				t.status = PortBusy
				break
			}
			// Start the command in a goroutine.
			t.cmd, err = t.config.getCommand(ctx)
			if err != nil {
				ch <- err
				break
			}
			go func() {
				b, err := t.cmd.CombinedOutput()
				ch <- errors.Wrap(err, string(b))
			}()
			if t.status != Reopening {
				t.status = Opening
				break
			}
			fallthrough
		// Transition state for the table rendering
		case Opening:
			t.status = Open
			t.err = nil
			t.startedAt = time.Now()
		case Error, Signal:
			t.status = Reopening
		}
		m.Unlock()
		time.Sleep(2 * time.Second)
	}
}
