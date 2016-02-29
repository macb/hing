package config

import (
	"io/ioutil"
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
)

func testDir(t *testing.T) (string, func()) {
	dir, err := ioutil.TempDir("", "haproxy_config")
	if err != nil {
		t.Fatal("failed to build dir")
	}
	return dir, func() { defer os.RemoveAll(dir) }
}

func TestNewConfig(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	c := NewConfig(&testclient.FakeIngress{}, dir+"/file", "local.cluster")
	if c == nil {
		t.Fatal("failed to build Config")
	}
}

func TestCanonicalizedName(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		host      string
		path      string
		expected  string
	}{
		{
			name:      "base path example",
			namespace: "default",
			host:      "google.com",
			path:      "/",
			expected:  "default_google_com",
		},
		{
			name:      "extended path example",
			namespace: "default",
			host:      "google.com",
			path:      "/my/path",
			expected:  "default_google_com_my_path",
		},
		{
			name:      "host with dashes",
			namespace: "default",
			host:      "fake-google.com",
			path:      "/my/path",
			expected:  "default_fake_google_com_my_path",
		},
	}

	for i, test := range tests {
		outcome := canonicalizedName(test.namespace, test.host, test.path)
		if outcome != test.expected {
			t.Logf("%d: %s", i+1, test.name)
			t.Logf("want: %s", test.expected)
			t.Logf(" got: %s", outcome)
			t.Error("outcome did not match expected")
		}
	}
}
