// Integration tests against real postgres containers on a private docker network.
// They need Docker and are skipped when it is unavailable; `go test -short` skips
// them too. testPg plays the tools host + `local` endpoint; testPg2 is a remote
// server reachable from inside testPg by its network name.
package postgres

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/phuthuycoding/dbclone/internal/clone"
	"github.com/phuthuycoding/dbclone/internal/driver"
	"github.com/phuthuycoding/dbclone/internal/preflight"
)

const (
	testPg       = "dbclone-test-pg"
	testPg2      = "dbclone-test-pg2"
	testNet      = "dbclone-test-net"
	testPassword = "testpgpw"
	testImage    = "postgres:17-alpine"
)

var dockerOK bool

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}
	dockerOK = setup()
	code := m.Run()
	if dockerOK {
		exec.Command("docker", "rm", "-f", testPg, testPg2).Run()
		exec.Command("docker", "network", "rm", testNet).Run()
	}
	os.Exit(code)
}

// setup starts the two test containers on their own network; false means docker
// is unusable and every integration test skips.
func setup() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	if err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Run(); err != nil {
		return false
	}
	exec.Command("docker", "rm", "-f", testPg, testPg2).Run()
	exec.Command("docker", "network", "rm", testNet).Run()
	if out, err := exec.Command("docker", "network", "create", testNet).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "docker network create: %v\n%s", err, out)
		return false
	}
	for _, name := range []string{testPg, testPg2} {
		run := exec.Command("docker", "run", "-d", "--name", name,
			"--network", testNet, "-e", "POSTGRES_PASSWORD="+testPassword, testImage)
		if out, err := run.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "start %s: %v\n%s", testImage, err, out)
			return false
		}
	}
	for range 60 {
		if err := exec.Command("docker", "exec", testPg, "pg_isready", "-U", "postgres").Run(); err == nil {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

func pg(t *testing.T) {
	t.Helper()
	if !dockerOK {
		t.Skip("docker or the test containers are unavailable")
	}
	t.Setenv("DBCLONE_POSTGRES_CONTAINER", testPg)
}

// psql runs SQL inside a test container for seeding and verification.
func psql(t *testing.T, container, db, sql string) string {
	t.Helper()
	cmd := exec.Command("docker", "exec", "-e", "PGPASSWORD="+testPassword,
		container, "psql", "-U", "postgres", "-X", "-A", "-t",
		"-v", "ON_ERROR_STOP=1", "-d", db, "-c", sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("psql on %s %q: %v\n%s", container, sql, err, out)
	}
	return strings.TrimSpace(string(out))
}

func localEP() driver.Endpoint { return driver.Endpoint{Name: driver.LocalName, Local: true} }

func remoteEP(name, host, port, password string) driver.Endpoint {
	return driver.Endpoint{Name: name, Values: map[string]string{
		keyHost: host, keyPort: port, keyUser: "postgres", keyPassword: password,
	}}
}

// remoteSrc is the testPg2 server as a connection profile, reachable by its
// network name from inside the tools container.
func remoteSrc() driver.Endpoint { return remoteEP("src", testPg2, "5432", testPassword) }

type noopRep struct{}

func (noopRep) Start(int64)  {}
func (noopRep) Add(int64)    {}
func (noopRep) Stage(string) {}
func (noopRep) Active(int)   {}
func (noopRep) Done()        {}
func (noopRep) Fail()        {}

func runClone(t *testing.T, job clone.Job, src, dst driver.Endpoint, fresh bool) clone.Result {
	t.Helper()
	results := clone.Run(context.Background(), []clone.Job{job}, []clone.Reporter{noopRep{}}, clone.Options{
		Parallel: 4, Workers: 2, Fresh: fresh, Source: src, Target: dst, LogDir: t.TempDir(),
	})
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	return results[0]
}

// seedSource builds a database exercising every object kind: two schemas, an FK,
// a sequence default, index, trigger+function, view and materialized view.
func seedSource(t *testing.T, db string) {
	t.Helper()
	psql(t, testPg2, "postgres", `DROP DATABASE IF EXISTS `+db+` WITH (FORCE)`)
	psql(t, testPg2, "postgres", `CREATE DATABASE `+db)
	psql(t, testPg2, db, `
CREATE SCHEMA analytics;
CREATE TABLE public.customers (id serial PRIMARY KEY, name text);
CREATE TABLE public.orders (id serial PRIMARY KEY, customer_id int REFERENCES public.customers(id), note text);
CREATE INDEX orders_note_idx ON public.orders(note);
CREATE SEQUENCE analytics.seq;
CREATE TABLE analytics.events (id int DEFAULT nextval('analytics.seq'), kind text);
CREATE FUNCTION public.bump() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END; $$;
CREATE TRIGGER trg BEFORE INSERT ON public.orders FOR EACH ROW EXECUTE FUNCTION public.bump();
CREATE VIEW public.v_orders AS SELECT id, note FROM public.orders;
CREATE MATERIALIZED VIEW public.mv_orders AS SELECT count(*) AS c FROM public.orders;
INSERT INTO public.customers(name) VALUES ('a'),('b');
INSERT INTO public.orders(customer_id,note) VALUES (1,'x'),(2,'y');
INSERT INTO analytics.events(kind) VALUES ('e1');
REFRESH MATERIALIZED VIEW public.mv_orders;
`)
}

func TestPreflightPostgres(t *testing.T) {
	pg(t)
	report, err := preflight.Run(context.Background(), []driver.Driver{New()})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if len(report.Ready) != 1 || report.Ready[0].Name() != "postgres" {
		t.Fatalf("Ready = %+v, want the postgres driver", report.Ready)
	}
	t.Setenv("DBCLONE_POSTGRES_CONTAINER", "no-such-container")
	report, err = preflight.Run(context.Background(), []driver.Driver{New()})
	if err != nil || len(report.Ready) != 0 || !report.Failed() {
		t.Fatalf("bogus container: Ready=%v err=%v, want a failed check", report.Ready, err)
	}
}

func TestCloneWholeDatabase(t *testing.T) {
	pg(t)
	seedSource(t, "app")
	res := runClone(t, clone.Job{Driver: New(), DB: "app"}, remoteSrc(), localEP(), false)
	if res.Err != nil {
		t.Fatalf("clone failed: %v (log %s)", res.Err, res.Log)
	}
	checks := map[string]string{
		"orders rows":     `SELECT count(*) FROM public.orders`,
		"order notes":     `SELECT string_agg(note, ',' ORDER BY id) FROM public.orders`,
		"customer seq":    `SELECT last_value FROM public.customers_id_seq`,
		"events seq row":  `SELECT id FROM analytics.events WHERE kind='e1'`,
		"view":            `SELECT count(*) FROM public.v_orders`,
		"matview data":    `SELECT c FROM public.mv_orders`,
		"function exists": `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.proname='bump'`,
		"trigger exists":  `SELECT count(*) FROM pg_trigger WHERE tgname='trg' AND NOT tgisinternal`,
		"index exists":    `SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname='orders_note_idx'`,
		"fk exists":       `SELECT count(*) FROM pg_constraint WHERE conname='orders_customer_id_fkey'`,
	}
	want := map[string]string{
		"orders rows": "2", "order notes": "x,y", "customer seq": "2",
		"events seq row": "1", "view": "2", "matview data": "2",
		"function exists": "1", "trigger exists": "1", "index exists": "1", "fk exists": "1",
	}
	for name, sql := range checks {
		if got := psql(t, testPg, "app", sql); got != want[name] {
			t.Errorf("%s = %q, want %q", name, got, want[name])
		}
	}
	// Re-run: drop-and-recreate must make a second clone clean.
	res = runClone(t, clone.Job{Driver: New(), DB: "app"}, remoteSrc(), localEP(), false)
	if res.Err != nil {
		t.Fatalf("second clone failed: %v", res.Err)
	}
}

func TestCloneSelectedObjects(t *testing.T) {
	pg(t)
	seedSource(t, "app2")
	job := clone.Job{Driver: New(), DB: "app2", Only: []string{"public.orders"}}
	res := runClone(t, job, remoteSrc(), localEP(), false)
	if res.Err != nil {
		t.Fatalf("clone failed: %v", res.Err)
	}
	if got := psql(t, testPg, "app2", `SELECT count(*) FROM public.orders`); got != "2" {
		t.Errorf("orders rows = %q, want 2", got)
	}
	if got := psql(t, testPg, "app2", `SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname='orders_note_idx'`); got != "1" {
		t.Error("index on selected table missing")
	}
	if got := psql(t, testPg, "app2", `SELECT count(*) FROM pg_trigger WHERE tgname='trg' AND NOT tgisinternal`); got != "1" {
		t.Error("trigger on selected table missing (its function was not restored)")
	}
	if got := psql(t, testPg, "app2", `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='customers'`); got != "0" {
		t.Error("unselected table public.customers was created")
	}
	if got := psql(t, testPg, "app2", `SELECT count(*) FROM pg_constraint WHERE conname='orders_customer_id_fkey'`); got != "0" {
		t.Error("FK to unselected table was created")
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "orders_customer_id_fkey") {
			found = true
		}
	}
	if !found {
		t.Errorf("no warning names the skipped FK: %v", res.Warnings)
	}
}

func TestPrepareFresh(t *testing.T) {
	pg(t)
	psql(t, testPg, "postgres", `DROP DATABASE IF EXISTS freshdb WITH (FORCE)`)
	psql(t, testPg, "postgres", `CREATE DATABASE freshdb`)
	psql(t, testPg, "freshdb", `CREATE TABLE keepme (id int)`)
	d := New()
	dst := localEP()
	if err := d.Prepare(context.Background(), dst, "freshdb", false); err != nil {
		t.Fatalf("Prepare(fresh=false): %v", err)
	}
	if got := psql(t, testPg, "freshdb", `SELECT count(*) FROM keepme`); got != "0" {
		t.Errorf("fresh=false changed the database (keepme rows = %q)", got)
	}
	// Hold an idle connection so a plain DROP would hang.
	hold := exec.Command("docker", "exec", "-d", testPg,
		"psql", "-U", "postgres", "-d", "freshdb", "-c", "SELECT pg_sleep(60)")
	if out, err := hold.CombinedOutput(); err != nil {
		t.Fatalf("hold connection: %v\n%s", err, out)
	}
	time.Sleep(time.Second)
	if err := d.Prepare(context.Background(), dst, "freshdb", true); err != nil {
		t.Fatalf("Prepare(fresh=true) with an open connection: %v", err)
	}
	if got := psql(t, testPg, "freshdb", `SELECT count(*) FROM information_schema.tables WHERE table_schema='public'`); got != "0" {
		t.Errorf("fresh did not drop (tables = %q)", got)
	}
}

func TestCloneRemoteToRemote(t *testing.T) {
	pg(t)
	seedSource(t, "app3")
	src := remoteEP("a", testPg2, "5432", testPassword)
	dst := remoteEP("b", testPg, "5432", testPassword)
	res := runClone(t, clone.Job{Driver: New(), DB: "app3"}, src, dst, false)
	if res.Err != nil {
		t.Fatalf("remote-to-remote clone failed: %v", res.Err)
	}
	if got := psql(t, testPg, "app3", `SELECT count(*) FROM public.orders`); got != "2" {
		t.Errorf("orders rows on remote target = %q, want 2", got)
	}
	if got := psql(t, testPg, "app3", `SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname='orders_note_idx'`); got != "1" {
		t.Error("index missing on remote target")
	}
}

func TestPermanentAuthFailure(t *testing.T) {
	pg(t)
	bad := remoteEP("bad", testPg2, "5432", "wrongpw")
	_, err := New().Databases(context.Background(), bad)
	if err == nil {
		t.Fatal("Databases with a wrong password did not fail")
	}
	if !New().Permanent(err.Error()) {
		t.Errorf("auth failure not classified permanent: %v", err)
	}
	if strings.Contains(err.Error(), "wrongpw") {
		t.Error("password leaked into the error output")
	}
}
