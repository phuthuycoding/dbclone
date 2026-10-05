// Package config loads and saves the staging connection settings (.env.staging). Settings
// live in the process environment so the drivers can read them and hand them to docker.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/phuthuycoding/dbclone/internal/driver"
)

// Load reads KEY=VALUE lines; variables already set in the environment win. A missing file
// is not an error: it reports found=false so the caller can onboard.
func Load(path string) (found bool, err error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(strings.TrimPrefix(k, "export ")), strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, v)
		}
	}
	return true, sc.Err()
}

// Save writes every driver field to path with mode 0600 — the file holds passwords.
func Save(path string, drivers []driver.Driver) error {
	var b strings.Builder
	b.WriteString("# dbclone staging connections — contains secrets, do not commit\n")
	for _, d := range drivers {
		fmt.Fprintf(&b, "\n# %s\n", d.Name())
		for _, f := range d.Fields() {
			fmt.Fprintf(&b, "%s=\"%s\"\n", f.Key, os.Getenv(f.Key))
		}
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600) // WriteFile keeps the mode of an existing file
}
