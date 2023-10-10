// Package internal contains everything you do not want to expose.
package internal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var signalRegex = regexp.MustCompile(`signal: ([a-z ]+)$`)

// K8sInfo contains all information required to use a kubectl port forward command.
type K8sInfo struct {
	Context   string `json:"context"`
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

//nolint:gosec // I'm happy for now.
func (c *TunnelConfig) getCommand(ctx context.Context) (*exec.Cmd, error) {
	if c.K8s != nil {
		args := []string{"port-forward", "-n", c.K8s.Namespace}
		if c.K8s.Context != "" {
			args = append(args, "--context", c.K8s.Context)
		}
		args = append(args, c.K8s.Service, fmt.Sprintf("%d:%d", c.LocalPort, c.K8s.Port))
		return exec.CommandContext(ctx, "kubectl", args...), nil
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
	err := syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
	if err != nil && !errors.Is(err, os.ErrProcessDone) && err.Error() != "no such process" {
		fmt.Printf("Error while killing %s: %v\n", t.config.Name, err)
	}
}

// GetName returns the name of the tunnel as specified in its config.
func (t *Tunnel) GetName() string {
	return t.config.Name
}

// GetType returns the config type being used by this tunnel. See the
// description of TunnelConfig to know which ones are available.
func (t *Tunnel) GetType() string {
	if t.config.K8s != nil {
		return "k8s"
	}
	if t.config.Custom != "" {
		return "custom"
	}
	return "N/A"
}

// GetLocalPort returns the local port used by the tunnel as specified in its
// config.
func (t *Tunnel) GetLocalPort() int {
	return t.config.LocalPort
}

//nolint:gosec // I'm happy for now.
func isPortBusy(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true // Assume port is busy
	}
	l.Close() // nolint:errcheck // should be fine.
	return false
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
			if isPortBusy(t.config.LocalPort) {
				t.status = PortBusy
				break
			}
			// Start the command in a goroutine.
			t.cmd, err = t.config.getCommand(ctx)
			if err != nil {
				ch <- fmt.Errorf("get command: %w", err)
				break
			}
			t.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			go func() {
				b, err := t.cmd.CombinedOutput()
				ch <- fmt.Errorf("%s: %w", string(b), err)
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
