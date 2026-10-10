package postgres

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/phuthuycoding/dbclone/internal/driver"
)

func TestLocal(t *testing.T) {
	d := New()
	l := d.Local()
	if l.Container != "postgres" || l.ContainerEnv != "DBCLONE_POSTGRES_CONTAINER" {
		t.Fatalf("Local() = %+v, want container postgres overridable by DBCLONE_POSTGRES_CONTAINER", l)
	}
	for _, tool := range []string{"pg_dump", "pg_restore", "psql"} {
		if !strings.Contains(strings.Join(l.Tools, " "), tool) {
			t.Errorf("tools %v missing %q", l.Tools, tool)
		}
	}
	if len(l.Credentials) != 1 || l.Credentials[0] != "POSTGRES_PASSWORD" {
		t.Errorf("Credentials = %v, want only POSTGRES_PASSWORD (POSTGRES_USER is optional)", l.Credentials)
	}
	if !strings.Contains(l.RunExample, "postgres:") {
		t.Errorf("RunExample %q does not run the official image", l.RunExample)
	}
	t.Setenv("DBCLONE_POSTGRES_CONTAINER", "mypg")
	if got := New().Local().Container; got != "mypg" {
		t.Errorf("container = %q, want mypg", got)
	}
}

func TestFieldsConfiguredAddress(t *testing.T) {
	d := New()
	keys := map[string]driver.Field{}
	for _, f := range d.Fields() {
		keys[f.Key] = f
	}
	for _, k := range []string{keyHost, keyPort, keyUser, keyPassword} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("Fields() missing %q", k)
		}
	}
	if keys[keyPort].Default != "5432" || !keys[keyPassword].Secret {
		t.Errorf("port default or password secret wrong: %+v", keys)
	}
	local := driver.Endpoint{Name: driver.LocalName, Local: true}
	if !d.Configured(local) {
		t.Error("local endpoint must always be configured")
	}
	ep := driver.Endpoint{Name: "p", Values: map[string]string{keyHost: "HOST", keyUser: "u"}}
	if !d.Configured(ep) {
		t.Error("host+user should configure")
	}
	for _, missing := range []driver.Endpoint{
		{Name: "a", Values: map[string]string{keyUser: "u"}},
		{Name: "b", Values: map[string]string{keyHost: "h"}},
	} {
		if d.Configured(missing) {
			t.Errorf("Configured(%v) = true, want false", missing.Values)
		}
	}
	if got, want := d.Address(ep), "host:5432"; got != want {
		t.Errorf("Address = %q, want %q", got, want)
	}
	if d.Address(local) != driver.LocalName {
		t.Error("local address must be the built-in name")
	}
}

func TestSafeName(t *testing.T) {
	for _, ok := range []string{"public.orders", "analytics.events_2", "s.a.b", "t", "sch_ema.t_1"} {
		if !safeName.MatchString(ok) {
			t.Errorf("safeName(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"public.Orders", `weird"name`, "a;b", "a b", ".x", "x.", ""} {
		if safeName.MatchString(bad) {
			t.Errorf("safeName(%q) = true, want false", bad)
		}
	}
}

func TestWholeStream(t *testing.T) {
	p := pipe{db: "app"}
	s := p.whole(4, 1024)
	if s.Weight != 4 || s.Size != 1024 {
		t.Fatalf("stream weight/size = %d/%d, want 4/1024", s.Weight, s.Size)
	}
	dump := script(t, s.Dump(context.Background()))
	for _, want := range []string{"pg_dump", "-Fd", `--dbname="$DB"`, `-f "$STAGE"`, `rm -rf "$STAGE"`} {
		if !strings.Contains(dump, want) {
			t.Errorf("dump %q missing %q", dump, want)
		}
	}
	if strings.Contains(dump, "testpw") {
		t.Error("password on dump argv")
	}
	restore := script(t, s.Restore(context.Background()))
	for _, want := range []string{"cat >/dev/null", "pg_restore", `-j "$WORKERS"`, "--clean", "--if-exists", "--no-owner", "--no-privileges", "--verbose", `"$STAGE"`, `rm -rf "$STAGE"`, `-d"$DB"`} {
		if !strings.Contains(restore, want) {
			t.Errorf("restore %q missing %q", restore, want)
		}
	}
}

func TestTableArgs(t *testing.T) {
	got := tableArgs([]string{"public.orders", "analytics.events"}, []string{"public.orders_id_seq", `bad"name`})
	want := " -t public.orders -t analytics.events -t public.orders_id_seq"
	if got != want {
		t.Errorf("tableArgs = %q, want %q", got, want)
	}
}

func TestAwkFilter(t *testing.T) {
	if _, err := exec.LookPath("awk"); err != nil {
		t.Skip("awk not available")
	}
	sql := `
ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id);

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES analytics.parents(id);

CREATE INDEX orders_note_idx ON public.orders USING btree (note);

CREATE TRIGGER trg BEFORE INSERT ON public.orders FOR EACH ROW EXECUTE FUNCTION public.bump();
`
	cmd := exec.Command("awk", "-v", "RS=;", "-v", "ORS=;", "-v", "DROP_REFS= analytics.parents ", "{"+awkFilter+"}")
	cmd.Stdin = strings.NewReader(sql)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("awk: %v", err)
	}
	got := string(out)
	if strings.Contains(got, "orders_parent_id_fkey") {
		t.Error("FK to unselected analytics.parents was not dropped")
	}
	for _, want := range []string{"orders_customer_id_fkey", "orders_note_idx", "CREATE TRIGGER trg"} {
		if !strings.Contains(got, want) {
			t.Errorf("filtered output missing %q:\n%s", want, got)
		}
	}
}

func TestPermanent(t *testing.T) {
	for _, yes := range []string{
		`psql: error: FATAL:  password authentication failed for user "r"`,
		`FATAL:  no pg_hba.conf entry for host "10.0.0.1"`,
		`ERROR:  permission denied for table orders`,
		`ERROR:  must be owner of table orders`,
		`FATAL:  database "app" does not exist`,
		`pg_restore: error: relation "public.orders" does not exist`,
		`pg_dump: error: aborting because of server version mismatch`,
	} {
		if !New().Permanent(yes) {
			t.Errorf("Permanent(%q) = false, want true", yes)
		}
	}
	for _, no := range []string{
		`server closed the connection unexpectedly`,
		`could not connect to server: Connection refused`,
		`pg_restore: error: timeout expired`,
	} {
		if New().Permanent(no) {
			t.Errorf("Permanent(%q) = true, want false", no)
		}
	}
}

func TestStageMarkers(t *testing.T) {
	d := New()
	for in, want := range map[string]string{
		`pg_restore: processing data for table "public"."orders"`: "public.orders",
		`pg_restore: processing data for table public.orders`:     "public.orders",
		`pg_restore: creating TABLE "public.orders"`:              "",
	} {
		if got := d.StageFromLog(in); got != want {
			t.Errorf("StageFromLog(%q) = %q, want %q", in, got, want)
		}
	}
	chunk := []byte("\nCOPY public.orders (id, note) FROM stdin;\n1\tx\n")
	if got := d.StageFromStream(chunk); got != "public.orders" {
		t.Errorf("StageFromStream = %q, want public.orders", got)
	}
	if got := d.StageFromStream([]byte("\x00\x01binary")); got != "" {
		t.Errorf("StageFromStream(binary) = %q, want empty", got)
	}
}

// script extracts the `sh -c` payload of a docker exec command for inspection.
func script(t *testing.T, cmd *exec.Cmd) string {
	t.Helper()
	if len(cmd.Args) == 0 {
		t.Fatal("empty command")
	}
	return cmd.Args[len(cmd.Args)-1]
}
