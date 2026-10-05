package docker

import "testing"

func TestRedact(t *testing.T) {
	cases := map[string]string{
		"failed to connect to mongodb://reader:s3cr3t@db.example.com:27017/?authSource=shop: x": "failed to connect to mongodb://***@db.example.com:27017/?authSource=shop: x",
		"mongodb+srv://u:p%40x@c.net/?a=b":    "mongodb+srv://***@c.net/?a=b",
		"mongodb://u:p@ss@h:27017/db":         "mongodb://***@h:27017/db",
		"mysql://root:pw@h:3306/db":           "mysql://***@h:3306/db",
		"no credentials mongodb://h:27017/db": "no credentials mongodb://h:27017/db",
		"Access denied for user 'reader'@'%'": "Access denied for user 'reader'@'%'",
	}
	for in, want := range cases {
		if got := Redact(in); got != want {
			t.Errorf("Redact(%q) = %q, want %q", in, got, want)
		}
	}
}
