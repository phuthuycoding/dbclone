// Package ui is the terminal front: prompts (huh) and the live progress board (mpb).
package ui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/phuthuycoding/dbclone/internal/clone"
	"github.com/phuthuycoding/dbclone/internal/config"
	"github.com/phuthuycoding/dbclone/internal/driver"
)

// EditProfile asks for a profile's name and its connection settings for every engine,
// prefilled with p. Leaving an engine's fields empty means the profile has no such engine.
func EditProfile(drivers []driver.Driver, p *config.Profile, taken []string) error {
	var engines []string
	for _, d := range drivers {
		engines = append(engines, d.Name())
	}
	name := p.Name
	intro := huh.NewNote().
		Title("Connection profile").
		Description("A profile is a named connection — staging, prod, a teammate's server — that you can clone FROM or TO.\n\n" +
			"The local Docker containers are the built-in profile \"local\" and need nothing here.\n" +
			"Fill in the engines this server has (" + strings.Join(engines, ", ") + ") and leave the others empty.\n" +
			"The connection is tested before anything is copied.").
		Next(true)
	groups := []*huh.Group{huh.NewGroup(intro, huh.NewInput().Title("Profile name").
		Placeholder("staging").
		Validate(func(v string) error {
			v = strings.TrimSpace(v)
			if err := config.ValidName(v); err != nil {
				return err
			}
			if v != p.Name && slices.Contains(taken, v) {
				return fmt.Errorf("a profile %q already exists", v)
			}
			return nil
		}).
		Value(&name))}
	values := map[string]*string{}
	for _, d := range drivers {
		var fields []huh.Field
		for _, f := range d.Fields() {
			v := p.Values[f.Key]
			if v == "" {
				v = f.Default
			}
			values[f.Key] = &v
			in := huh.NewInput().Title(f.Title).Description(f.Description).
				Placeholder(f.Placeholder).Value(values[f.Key])
			if f.Secret {
				in = in.EchoMode(huh.EchoModePassword)
			}
			fields = append(fields, in)
		}
		groups = append(groups, huh.NewGroup(fields...).Title(d.Name()))
	}
	if err := huh.NewForm(groups...).Run(); err != nil {
		return err
	}
	p.Name = strings.TrimSpace(name)
	p.Values = map[string]string{}
	for k, v := range values {
		if v := strings.TrimSpace(*v); v != "" {
			p.Values[k] = v
		}
	}
	return nil
}

// ManageProfiles lets the user add, edit and delete profiles until they choose Done.
func ManageProfiles(store *config.Store, drivers []driver.Driver) error {
	for {
		const add, done = "\x00add", "\x00done"
		opts := []huh.Option[string]{huh.NewOption("+ Add a profile", add)}
		for _, n := range store.Names() {
			opts = append(opts, huh.NewOption("Edit or delete "+n, n))
		}
		opts = append(opts, huh.NewOption("Done", done))
		var pick string
		err := huh.NewSelect[string]().
			Title("Connection profiles").
			Description(fmt.Sprintf("saved in %s · \"local\" (your Docker containers) is built in", store.Path)).
			Options(opts...).Value(&pick).Run()
		if err != nil {
			return err
		}
		switch pick {
		case done:
			return nil
		case add:
			p := config.Profile{}
			if err := EditProfile(drivers, &p, store.Names()); err != nil {
				return err
			}
			store.Put("", p)
		default:
			action := "edit"
			if err := huh.NewSelect[string]().Title("Profile "+pick).
				Options(huh.NewOption("Edit", "edit"), huh.NewOption("Delete", "delete"), huh.NewOption("Back", "back")).
				Value(&action).Run(); err != nil {
				return err
			}
			switch action {
			case "edit":
				p, _ := store.Get(pick)
				if err := EditProfile(drivers, &p, store.Names()); err != nil {
					return err
				}
				store.Put(pick, p)
			case "delete":
				if ok, err := Confirm("Delete profile "+pick+"?", "", "Delete", "Keep"); err != nil || !ok {
					if err != nil {
						return err
					}
					continue
				}
				store.Delete(pick)
			default:
				continue
			}
		}
		if err := store.Save(); err != nil {
			return err
		}
	}
}

// SelectProfile asks which profile to use, with def preselected.
func SelectProfile(title string, names []string, def string) (string, error) {
	opts := make([]huh.Option[string], len(names))
	for i, n := range names {
		label := n
		if n == driver.LocalName {
			label += "  (your Docker containers)"
		}
		opts[i] = huh.NewOption(label, n)
	}
	pick := def
	err := huh.NewSelect[string]().Title(title).Options(opts...).Value(&pick).Run()
	return pick, err
}

// ConfirmTarget makes the user type the target's name before writing to a remote server.
func ConfirmTarget(target, summary string) error {
	var typed string
	return huh.NewInput().
		Title(fmt.Sprintf("You are about to WRITE to %q. Type %s to continue.", target, target)).
		Description(summary).
		Validate(func(v string) error {
			if strings.TrimSpace(v) != target {
				return fmt.Errorf("type %s exactly, or press ctrl+c to cancel", target)
			}
			return nil
		}).
		Value(&typed).Run()
}

func Confirm(title, description, yes, no string) (bool, error) {
	ok := true
	err := huh.NewConfirm().Title(title).Description(description).
		Affirmative(yes).Negative(no).Value(&ok).Run()
	return ok, err
}

// Pick lets the user tick the databases to clone; existing marks the ones already on the target.
func Pick(available []clone.Job, existing map[string]bool, target string) ([]clone.Job, error) {
	opts := make([]huh.Option[int], 0, len(available))
	for i, j := range available {
		label := j.String()
		if existing[label] {
			label += "  (exists on " + target + ")"
		}
		opts = append(opts, huh.NewOption(label, i))
	}
	var idx []int
	err := huh.NewMultiSelect[int]().
		Title("Select databases to clone to " + target).
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
func Narrow(ctx context.Context, src driver.Endpoint, jobs []clone.Job) ([]clone.Job, error) {
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
		objs, err := j.Driver.Objects(ctx, src, j.DB)
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
