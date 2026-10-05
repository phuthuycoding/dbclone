package mongo

import "testing"

func TestServerURI(t *testing.T) {
	cases := map[string]string{
		"mongodb://u:p@h:27017/shop":                           "mongodb://u:p@h:27017/?authSource=shop",
		"mongodb://u:p@h1:27017,h2:27017/shop?replicaSet=rs":   "mongodb://u:p@h1:27017,h2:27017/?replicaSet=rs&authSource=shop",
		"mongodb://u:p@h/shop?authSource=admin":                "mongodb://u:p@h/?authSource=admin",
		"mongodb://u:p%40x@h/?authSource=admin":                "mongodb://u:p%40x@h/?authSource=admin",
		"mongodb://u:p@h:27017":                                "mongodb://u:p@h:27017",
		"mongodb+srv://u:p@cluster.x.net/app?retryWrites=true": "mongodb+srv://u:p@cluster.x.net/?retryWrites=true&authSource=app",
	}
	for in, want := range cases {
		if got := serverURI(in); got != want {
			t.Errorf("serverURI(%q) = %q, want %q", in, got, want)
		}
	}
}
