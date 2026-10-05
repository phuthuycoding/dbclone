package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTripAndLegacyImport(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, ".env.staging")
	os.WriteFile(legacy, []byte("# old\nSTG_MONGO_URI=\"mongodb://u:p@h/?authSource=admin\"\nSTG_MYSQL_HOST=db\nSTG_MYSQL_USER=reader\nSTG_MYSQL_PASSWORD=''\n"), 0o600)

	s, err := Load(filepath.Join(dir, "cfg", "profiles.json"))
	if err != nil || len(s.Profiles) != 0 {
		t.Fatalf("missing file must load empty: %v %v", s, err)
	}
	added, err := s.ImportLegacy(legacy)
	if err != nil || !added {
		t.Fatalf("import: added=%v err=%v", added, err)
	}
	if again, _ := s.ImportLegacy(legacy); again {
		t.Error("import must happen once")
	}
	st, err := os.Stat(s.Path)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("profiles file must be 0600: %v %v", st, err)
	}

	s2, err := Load(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	ep, err := s2.Endpoint("staging")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"mongo.uri": "mongodb://u:p@h/?authSource=admin", "mysql.host": "db", "mysql.user": "reader"}
	for k, v := range want {
		if ep.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, ep.Get(k), v)
		}
	}
	if _, ok := ep.Values["mysql.password"]; ok {
		t.Error("empty legacy values must not be imported")
	}
	if l, _ := s2.Endpoint("local"); !l.Local {
		t.Error("local must resolve to the local container")
	}
	if _, err := s2.Endpoint("prod"); err == nil {
		t.Error("unknown profile must fail")
	}

	s2.Put("staging", Profile{Name: "stg", Values: map[string]string{}})
	s2.Put("", Profile{Name: "prod"})
	if got := s2.Names(); len(got) != 2 || got[0] != "prod" || got[1] != "stg" {
		t.Errorf("rename/add: names = %v", got)
	}
	s2.Delete("prod")
	if got := s2.Names(); len(got) != 1 {
		t.Errorf("delete: names = %v", got)
	}
}

func TestValidName(t *testing.T) {
	for name, ok := range map[string]bool{"staging": true, "prod-eu_2": true, "local": false, "a b": false, "": false} {
		if (ValidName(name) == nil) != ok {
			t.Errorf("ValidName(%q) ok=%v", name, !ok)
		}
	}
}
