// Package docker runs shell snippets inside the local database containers, where the
// dump/restore tools already ship with a client version matching the local server.
package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

// Sh builds `docker exec [-i] -e NAME... <container> sh -c <script>`. Values travel in the
// docker client's own environment; only the variable names appear on argv, so secrets are
// never visible in `ps` of the docker command. Scripts must reference values as "$NAME".
func Sh(ctx context.Context, container, script string, stdin bool, env map[string]string) *exec.Cmd {
	args := []string{"exec"}
	if stdin {
		args = append(args, "-i")
	}
	cmdEnv := os.Environ()
	for k, v := range env {
		args = append(args, "-e", k)
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	args = append(args, container, "sh", "-c", script)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = cmdEnv
	return cmd
}

// credentials matches the user:password@ part of a connection string. Tools print the
// full URI in some errors (mongodump does on a failed handshake), so all tool output is
// passed through Redact before it is logged or shown.
var credentials = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/\s]+@`)

// Redact masks credentials in connection strings found in s.
func Redact(s string) string { return credentials.ReplaceAllString(s, "${1}***@") }

// Lines runs cmd and returns its non-empty stdout lines minus those in skip, sorted.
func Lines(cmd *exec.Cmd, skip ...string) ([]string, error) {
	out, err := Output(cmd)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" && !slices.Contains(skip, l) {
			lines = append(lines, l)
		}
	}
	slices.Sort(lines)
	return lines, nil
}

// Output runs cmd and returns trimmed stdout; stderr is folded into the error.
func Output(cmd *exec.Cmd) (string, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, Redact(strings.TrimSpace(stderr.String())))
	}
	return strings.TrimSpace(string(out)), nil
}
