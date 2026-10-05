// Package preflight checks the local machine before anything else happens: Docker, its
// daemon, and for every driver a running container with the right tools and credentials.
// Each failure comes with the command that fixes it.
package preflight

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/phuthuycoding/dbclone/internal/driver"
)

// Check is one line of the report.
type Check struct {
	OK     bool
	Title  string
	Detail string   // shown after the title, e.g. a version
	Fix    []string // what to do when it failed, one line each
}

// Report is the outcome of Run.
type Report struct {
	Checks []Check
	Ready  []driver.Driver // drivers whose local side is usable
}

// Failed reports whether any check failed.
func (r Report) Failed() bool {
	for _, c := range r.Checks {
		if !c.OK {
			return true
		}
	}
	return false
}

// ErrNoDocker means Docker itself is unusable, so no driver can work.
var ErrNoDocker = errors.New("docker is not usable")

// Run checks Docker and then every driver's container. A missing container only disables
// that driver; Docker being unusable is fatal and returns ErrNoDocker.
func Run(ctx context.Context, drivers []driver.Driver) (Report, error) {
	var r Report
	if _, err := exec.LookPath("docker"); err != nil {
		r.Checks = append(r.Checks, Check{Title: "Docker is not installed (no `docker` on PATH)", Fix: []string{
			"dbclone runs the database tools inside your local containers, so it needs Docker:",
			"https://docs.docker.com/get-docker/",
		}})
		return r, ErrNoDocker
	}
	version, err := output(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		fix := []string{"start Docker Desktop, or on Linux: sudo systemctl start docker"}
		if strings.Contains(strings.ToLower(err.Error()), "permission denied") {
			fix = []string{
				"your user cannot talk to the Docker daemon; on Linux:",
				"sudo usermod -aG docker $USER   (then log out and back in)",
			}
		}
		r.Checks = append(r.Checks, Check{Title: "Docker daemon is not reachable", Detail: firstLine(err.Error()), Fix: fix})
		return r, ErrNoDocker
	}
	r.Checks = append(r.Checks, Check{OK: true, Title: "Docker", Detail: version})

	for _, d := range drivers {
		c := checkLocal(ctx, d.Name(), d.Local())
		r.Checks = append(r.Checks, c)
		if c.OK {
			r.Ready = append(r.Ready, d)
		}
	}
	return r, nil
}

func checkLocal(ctx context.Context, engine string, l driver.Local) Check {
	title := fmt.Sprintf("%s: local container %q", engine, l.Container)
	override := fmt.Sprintf("or use an existing container:  %s=<name> dbclone", l.ContainerEnv)

	running, err := output(ctx, "docker", "inspect", "-f", "{{.State.Running}}", l.Container)
	switch {
	case err != nil:
		return Check{Title: title + " not found", Fix: []string{"create one:  " + l.RunExample, override}}
	case running != "true":
		return Check{Title: title + " is not running", Fix: []string{"start it:  docker start " + l.Container}}
	}

	// Names only — values are never read back.
	script := `for t in $TOOLS; do command -v "$t" >/dev/null 2>&1 || echo "tool $t"; done
for v in $VARS; do printenv "$v" >/dev/null 2>&1 || echo "var $v"; done`
	cmd := exec.CommandContext(ctx, "docker", "exec", "-e", "TOOLS", "-e", "VARS", l.Container, "sh", "-c", script)
	cmd.Env = append(cmd.Environ(), "TOOLS="+strings.Join(l.Tools, " "), "VARS="+strings.Join(l.Credentials, " "))
	out, err := cmd.Output()
	if err != nil {
		return Check{Title: title + " cannot be inspected", Detail: firstLine(err.Error())}
	}
	var tools, vars []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		kind, name, _ := strings.Cut(line, " ")
		switch kind {
		case "tool":
			tools = append(tools, name)
		case "var":
			vars = append(vars, name)
		}
	}
	var fix []string
	if len(tools) > 0 {
		fix = append(fix, "missing tools: "+strings.Join(tools, ", ")+" (not the official image?)")
	}
	if len(vars) > 0 {
		fix = append(fix, "missing env vars: "+strings.Join(vars, ", ")+" (dbclone logs in as root with them)")
	}
	if len(fix) > 0 {
		fix = append(fix, "recreate it from the official image:  "+l.RunExample, override)
		return Check{Title: title + " is not usable", Fix: fix}
	}
	return Check{OK: true, Title: title, Detail: "running"}
}

func output(ctx context.Context, name string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", errors.New(msg)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	return s
}
