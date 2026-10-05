package main

import (
	"reflect"
	"testing"

	"github.com/phuthuycoding/dbclone/internal/clone"
	"github.com/phuthuycoding/dbclone/internal/driver/mongo"
	"github.com/phuthuycoding/dbclone/internal/driver/mysql"
)

func TestParseOnly(t *testing.T) {
	mg, my := mongo.New(), mysql.New()
	available := []clone.Job{{Driver: mg, DB: "shop"}, {Driver: my, DB: "app"}}
	got := func(spec string) map[string][]string {
		jobs, err := parseOnly(spec, available)
		if err != nil {
			t.Fatalf("%s: %v", spec, err)
		}
		m := map[string][]string{}
		for _, j := range jobs {
			m[j.String()] = j.Only
		}
		return m
	}
	cases := map[string]map[string][]string{
		"mongo:shop":                            {"mongo:shop": nil},
		"mongo:shop.users,mongo:shop.orders":    {"mongo:shop": {"users", "orders"}},
		"mongo:shop.users,mongo:shop":           {"mongo:shop": nil},
		"mongo:shop.a.b, mysql:app.accounts":    {"mongo:shop": {"a.b"}, "mysql:app": {"accounts"}},
		"mysql:app.accounts,mysql:app.accounts": {"mysql:app": {"accounts"}},
	}
	for spec, want := range cases {
		if g := got(spec); !reflect.DeepEqual(g, want) {
			t.Errorf("parseOnly(%q) = %v, want %v", spec, g, want)
		}
	}
	if _, err := parseOnly("mongo:nope.x", available); err == nil {
		t.Error("unknown database must fail")
	}
}
