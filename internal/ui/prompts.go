// Package ui is the terminal front: prompts (huh) and the live progress board (mpb).
package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/phuthuycoding/dbclone/internal/clone"
	"github.com/phuthuycoding/dbclone/internal/driver"
)

// Onboard asks for every driver's staging settings, prefilled from the environment, and
// writes the answers back into it. Leaving a driver's fields blank skips that engine.
func Onboard(drivers []driver.Driver) error {
	var groups []*huh.Group
	values := map[string]*string{}
	for _, d := range drivers {
		var fields []huh.Field
		for _, f := range d.Fields() {
			v := driver.Env(f.Key, f.Default)
			values[f.Key] = &v
			in := huh.NewInput().Title(f.Title).Description(f.Description).
				Placeholder(f.Placeholder).Value(values[f.Key])
			if f.Secret {
				in = in.EchoMode(huh.EchoModePassword)
			}
			fields = append(fields, in)
		}
		groups = append(groups, huh.NewGroup(fields...).Title("Staging connection — "+d.Name()))
	}
	if err := huh.NewForm(groups...).Run(); err != nil {
		return err
	}
	for k, v := range values {
		os.Setenv(k, strings.TrimSpace(*v))
	}
	return nil
}

func Confirm(title, description, yes, no string) (bool, error) {
	ok := true
	err := huh.NewConfirm().Title(title).Description(description).
		Affirmative(yes).Negative(no).Value(&ok).Run()
	return ok, err
}

// Pick lets the user tick the databases to clone; local marks the ones that already exist.
func Pick(available []clone.Job, local map[string]bool) ([]clone.Job, error) {
	opts := make([]huh.Option[int], 0, len(available))
	for i, j := range available {
		label := j.String()
		if local[label] {
			label += "  (exists locally)"
		}
		opts = append(opts, huh.NewOption(label, i))
	}
	var idx []int
	err := huh.NewMultiSelect[int]().
		Title("Select databases to clone to local").
		Description("space: toggle · /: filter · enter: confirm").
		Options(opts...).
		Filterable(true).
		Height(min(len(opts)+4, 20)).
		Value(&idx).
		Run()
	jobs := make([]clone.Job, 0, len(idx))
	for _, i := range idx {
		jobs = append(jobs, available[i])
	}
	return jobs, err
}

// Narrow lets the user restrict some of the picked databases to specific tables or
// collections. Databases left unticked in the first step are cloned whole.
func Narrow(ctx context.Context, jobs []clone.Job) ([]clone.Job, error) {
	opts := make([]huh.Option[int], len(jobs))
	for i, j := range jobs {
		opts[i] = huh.NewOption(j.String(), i)
	}
	var idx []int
	err := huh.NewMultiSelect[int]().
		Title("Pick specific tables/collections for which databases?").
		Description("leave empty to clone every selected database whole · space: toggle · enter: confirm").
		Options(opts...).
		Height(min(len(opts)+4, 20)).
		Value(&idx).
		Run()
	if err != nil {
		return nil, err
	}
	for _, i := range idx {
		j := &jobs[i]
		objs, err := j.Driver.Objects(ctx, j.DB)
		if err != nil {
			return nil, fmt.Errorf("list objects of %s: %w", j, err)
		}
		opts := make([]huh.Option[string], len(objs))
		for k, o := range objs {
			label := fmt.Sprintf("%s  (%s", o.Name, o.Kind)
			if o.Size > 0 {
				label += fmt.Sprintf(", % .1f", decor.SizeB1024(o.Size))
			}
			opts[k] = huh.NewOption(label+")", o.Name).Selected(true)
		}
		var names []string
		err = huh.NewMultiSelect[string]().
			Title("Tables/collections to clone from " + j.String()).
			Description("all are ticked · space: toggle · /: filter · ctrl+a: toggle all · enter: confirm").
			Options(opts...).
			Filterable(true).
			Height(min(len(opts)+4, 20)).
			Validate(func(v []string) error {
				if len(v) == 0 {
					return errors.New("select at least one")
				}
				return nil
			}).
			Value(&names).
			Run()
		if err != nil {
			return nil, err
		}
		if len(names) < len(objs) {
			j.Only = names
		}
	}
	return jobs, nil
}
