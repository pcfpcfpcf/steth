// Package launch starts and stops engine processes from the profiles in
// steth's config file. It runs each profile's argv as given, with no shell,
// stops only the processes it started, and keeps each one's recent output.
package launch

import (
	"fmt"
	"maps"
	"net"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// stopGrace is how long a child has to exit after an interrupt before
	// it is killed.
	stopGrace = 5 * time.Second
	// pipeGrace is how long a child's output is still read after it exits,
	// while something it started holds its stdout open.
	pipeGrace       = time.Second
	portFlag        = "--port"
	loopback        = "127.0.0.1"
	portDialTimeout = 300 * time.Millisecond
	maxPort         = 65535
	stoppedExit     = "stopped"
)

// Launcher starts and stops the profiles' processes. It is safe for
// concurrent use.
type Launcher struct {
	profiles []Profile
	grace    time.Duration

	mu sync.Mutex
	// runs is each profile's last run, by name; it stays after the child
	// exits, until the profile starts again.
	runs map[string]*run
}

type run struct {
	model    string
	cmd      *exec.Cmd
	started  time.Time
	log      *lineLog
	stopping bool
	exit     string        // how the child ended; empty while it runs
	exited   chan struct{} // closed once exit is set
}

// Status is a profile and its child, if it has run.
type Status struct {
	Name    string     `json:"name"`
	Models  []string   `json:"models"`
	Running bool       `json:"running"`
	Model   string     `json:"model,omitempty"`
	PID     int        `json:"pid,omitempty"` // only while running
	Started *time.Time `json:"started,omitempty"`
	Exit    string     `json:"exit,omitempty"` // how the last child ended: "exit status 1", "stopped", ...
	Log     []string   `json:"log"`            // its last lines of stdout and stderr together, oldest first
}

func New(profiles []Profile) *Launcher {
	return &Launcher{profiles: profiles, grace: stopGrace, runs: map[string]*run{}}
}

// Start starts the named profile with model, which must be one of its
// Models, or "" when it has none. It refuses a profile already running
// under this launcher, and a command whose --port already answers on
// 127.0.0.1.
func (l *Launcher) Start(name, model string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	profile, err := l.profile(name)
	if err != nil {
		return err
	}
	if err := profile.checkModel(model); err != nil {
		return err
	}
	if r := l.runs[name]; r != nil && r.exit == "" {
		return fmt.Errorf("%q is already running as pid %d; stop it first", name, r.cmd.Process.Pid)
	}
	argv := profile.argv(model)
	if err := refuseAnsweringPort(argv); err != nil {
		return fmt.Errorf("not starting %q: %w", name, err)
	}
	r := &run{model: model, log: &lineLog{}, exited: make(chan struct{})}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = profile.Dir
	cmd.Env = environ(profile.Env)
	out := &lockedLog{mu: &l.mu, log: r.log}
	cmd.Stdout, cmd.Stderr = out, out
	cmd.WaitDelay = pipeGrace
	ownGroup(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %q: %w", name, err)
	}
	r.cmd, r.started = cmd, time.Now()
	l.runs[name] = r
	go l.wait(r)
	return nil
}

// Stop stops a child this launcher started, with the processes it started:
// on Unix it interrupts the child's process group, then kills the group
// after a grace period; on Windows it kills the process tree. It returns
// once the child's exit is recorded.
func (l *Launcher) Stop(name string) error {
	l.mu.Lock()
	if _, err := l.profile(name); err != nil {
		l.mu.Unlock()
		return err
	}
	r := l.runs[name]
	if r == nil || r.exit != "" {
		l.mu.Unlock()
		return fmt.Errorf("%q is not running; nothing to stop", name)
	}
	r.stopping = true
	l.mu.Unlock()
	if err := stopTree(r.cmd.Process, r.exited, l.grace); err != nil {
		return fmt.Errorf("stopping %q: %w", name, err)
	}
	return nil
}

// StopAll stops every child at once, for when steth exits.
func (l *Launcher) StopAll() {
	l.mu.Lock()
	var running []string
	for name, r := range l.runs {
		if r.exit == "" {
			running = append(running, name)
		}
	}
	l.mu.Unlock()
	var stopping sync.WaitGroup
	for _, name := range running {
		stopping.Add(1)
		go func() {
			defer stopping.Done()
			// The error is dropped because steth is exiting and has no one to
			// tell: it is a child that exited meanwhile, or one the OS will
			// not let steth signal.
			l.Stop(name)
		}()
	}
	stopping.Wait()
}

// Status is every profile, in config order, and its child if any.
func (l *Launcher) Status() []Status {
	l.mu.Lock()
	defer l.mu.Unlock()
	statuses := make([]Status, 0, len(l.profiles))
	for _, p := range l.profiles {
		s := Status{Name: p.Name, Models: append([]string{}, p.Models...), Log: []string{}}
		if r := l.runs[p.Name]; r != nil {
			started := r.started
			s.Running = r.exit == ""
			s.Model, s.Started, s.Exit, s.Log = r.model, &started, r.exit, r.log.lines()
			if s.Running {
				s.PID = r.cmd.Process.Pid
			}
		}
		statuses = append(statuses, s)
	}
	return statuses
}

func (l *Launcher) profile(name string) (Profile, error) {
	for _, p := range l.profiles {
		if p.Name == name {
			return p, nil
		}
	}
	names := make([]string, len(l.profiles))
	for i, p := range l.profiles {
		names[i] = p.Name
	}
	if len(names) == 0 {
		return Profile{}, fmt.Errorf(`no profile named %q; the config file has none, add them under "launch"`, name)
	}
	return Profile{}, fmt.Errorf("no profile named %q; the profiles are %s", name, quoteAll(names))
}

// wait records how the child ended. It must not hold l.mu while waiting:
// the child's output, which Wait drains, takes it.
func (l *Launcher) wait(r *run) {
	err := r.cmd.Wait()
	l.mu.Lock()
	r.log.finish()
	switch {
	case r.stopping:
		r.exit = stoppedExit
	case r.cmd.ProcessState != nil:
		r.exit = r.cmd.ProcessState.String()
	default:
		r.exit = err.Error()
	}
	l.mu.Unlock()
	close(r.exited)
}

// environ is steth's environment with env added; a name in env wins.
func environ(env map[string]string) []string {
	merged := os.Environ()
	for _, name := range slices.Sorted(maps.Keys(env)) {
		merged = append(merged, name+"="+env[name])
	}
	return merged
}

// refuseAnsweringPort says which port named in argv already answers.
func refuseAnsweringPort(argv []string) error {
	ports, err := namedPorts(argv)
	if err != nil {
		return err
	}
	for _, port := range ports {
		address := net.JoinHostPort(loopback, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", address, portDialTimeout)
		if err == nil {
			conn.Close()
			return fmt.Errorf("port %d already answers on %s; stop what listens there, or change the profile's %s", port, loopback, portFlag)
		}
	}
	return nil
}

// namedPorts reads every "--port N" and "--port=N" in argv.
func namedPorts(argv []string) ([]int, error) {
	var ports []int
	for i, arg := range argv {
		value, found := strings.CutPrefix(arg, portFlag+"=")
		if !found {
			if arg != portFlag || i+1 == len(argv) {
				continue
			}
			value = argv[i+1]
		}
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > maxPort {
			return nil, fmt.Errorf("%s %q is not a port; give a number from 1 to %d", portFlag, value, maxPort)
		}
		ports = append(ports, port)
	}
	return ports, nil
}
