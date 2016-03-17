package config

import (
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/util/intstr"
)

func testDir(t *testing.T) (string, func()) {
	dir, err := ioutil.TempDir("", "haproxy_config")
	if err != nil {
		t.Fatal("failed to build dir")
	}
	return dir, func() { os.RemoveAll(dir) }
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
			expected:  "default_fake_dash_google_com_my_path",
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

func TestCanonicalizedNamespaceHost(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		host      string
		expected  string
	}{
		{
			name:      "simple hostname",
			namespace: "default",
			host:      "foo",
			expected:  "default_foo",
		},
		{
			name:      "subdomain hostname",
			namespace: "default",
			host:      "foo.example",
			expected:  "default_foo_example",
		},
		{
			name:      "hostname with dash",
			namespace: "default",
			host:      "foo-example",
			expected:  "default_foo_dash_example",
		},
	}

	for i, test := range tests {
		outcome := canonicalizedNamespaceHost(test.namespace, test.host)
		if outcome != test.expected {
			t.Logf("%d: %s", i+1, test.name)
			t.Logf("want: %s", test.expected)
			t.Logf(" got: %s", outcome)
			t.Error("outcome did not match expected")
		}
	}
}

func TestCanonicalizedPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "base path example",
			path:     "/",
			expected: "",
		},
		{
			name:     "extended path example",
			path:     "/my/path",
			expected: "my_path",
		},
	}

	for i, test := range tests {
		outcome := canonicalizedPath(test.path)
		if outcome != test.expected {
			t.Logf("%d: %s", i+1, test.name)
			t.Logf("want: %s", test.expected)
			t.Logf(" got: %s", outcome)
			t.Error("outcome did not match expected")
		}
	}
}

func TestFeaturesFrom(t *testing.T) {
	tests := []struct {
		baseDomain string
		ingresses  []extensions.Ingress
		backends   []backend
		hostACLs   []acl
		frontends  []frontend
	}{
		{
			baseDomain: "example.com",
			ingresses: []extensions.Ingress{
				{
					ObjectMeta: api.ObjectMeta{
						Namespace: "default",
					},
					Spec: extensions.IngressSpec{
						Rules: []extensions.IngressRule{
							{
								Host: "foo",
								IngressRuleValue: extensions.IngressRuleValue{
									HTTP: &extensions.HTTPIngressRuleValue{
										Paths: []extensions.HTTPIngressPath{
											{
												Path: "/",
												Backend: extensions.IngressBackend{
													ServiceName: "foo",
													ServicePort: intstr.FromInt(3000),
												},
											},
										},
									},
								},
							},
							{
								Host: "invalid_host",
								IngressRuleValue: extensions.IngressRuleValue{
									HTTP: &extensions.HTTPIngressRuleValue{
										Paths: []extensions.HTTPIngressPath{
											{
												Path: "/",
												Backend: extensions.IngressBackend{
													ServiceName: "foo",
													ServicePort: intstr.FromInt(3000),
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: api.ObjectMeta{
						Namespace: "default",
					},
					Spec: extensions.IngressSpec{
						Rules: []extensions.IngressRule{
							{
								Host: "bar",
								IngressRuleValue: extensions.IngressRuleValue{
									HTTP: &extensions.HTTPIngressRuleValue{
										Paths: []extensions.HTTPIngressPath{
											{
												Path: "/my/path",
												Backend: extensions.IngressBackend{
													ServiceName: "bar",
													ServicePort: intstr.FromInt(9000),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			backends: []backend{
				{
					Name:   "default_foo",
					Server: "foo foo.default.svc.cluster.local:3000",
				},
				{
					Name:   "default_bar_my_path",
					Server: "bar bar.default.svc.cluster.local:9000",
				},
			},
			hostACLs: []acl{
				{
					Name:    "is_default_foo",
					Matcher: "hdr_beg(host) -i foo.example.com",
				},
				{
					Name:    "is_default_bar",
					Matcher: "hdr_beg(host) -i bar.example.com",
				},
			},
			frontends: []frontend{
				{
					HostACL: acl{
						Name:    "is_default_foo",
						Matcher: "hdr_beg(host) -i foo.example.com",
					},
					PathACL: acl{
						Name:    "is_default_foo_path",
						Matcher: "path_beg /",
					},
					Backend: backend{
						Name:   "default_foo",
						Server: "foo foo.default.svc.cluster.local:3000",
					},
				},
				{
					HostACL: acl{
						Name:    "is_default_bar",
						Matcher: "hdr_beg(host) -i bar.example.com",
					},
					PathACL: acl{
						Name:    "is_default_bar_my_path_path",
						Matcher: "path_beg /my/path",
					},
					Backend: backend{
						Name:   "default_bar_my_path",
						Server: "bar bar.default.svc.cluster.local:9000",
					},
				},
			},
		},
	}

	for _, test := range tests {
		backends, hostACLs, frontends := featuresFrom(test.ingresses, "example.com")
		if !reflect.DeepEqual(backends, test.backends) {
			t.Logf("want: %#v", test.backends)
			t.Logf(" got: %#v", backends)
			t.Fatal("unexpected backends")
		}

		if !reflect.DeepEqual(frontends, test.frontends) {
			t.Logf("want: %v", test.frontends)
			t.Logf(" got: %v", frontends)
			t.Fatal("unexpected frontends")
		}

		if !reflect.DeepEqual(hostACLs, test.hostACLs) {
			t.Logf("want: %v", test.hostACLs)
			t.Logf(" got: %v", hostACLs)
			t.Fatal("unexpected hostACLs")
		}
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		baseDomain string
		ingresses  []extensions.Ingress
		err        error
		changed    bool
		expected   string
	}{
		{
			baseDomain: "example.com",
			ingresses: []extensions.Ingress{
				{
					ObjectMeta: api.ObjectMeta{
						Namespace: "default",
					},
					Spec: extensions.IngressSpec{
						Rules: []extensions.IngressRule{
							{
								Host: "foo",
								IngressRuleValue: extensions.IngressRuleValue{
									HTTP: &extensions.HTTPIngressRuleValue{
										Paths: []extensions.HTTPIngressPath{
											{
												Path: "/",
												Backend: extensions.IngressBackend{
													ServiceName: "foo",
													ServicePort: intstr.FromInt(3000),
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: api.ObjectMeta{
						Namespace: "default",
					},
					Spec: extensions.IngressSpec{
						Rules: []extensions.IngressRule{
							{
								Host: "bar",
								IngressRuleValue: extensions.IngressRuleValue{
									HTTP: &extensions.HTTPIngressRuleValue{
										Paths: []extensions.HTTPIngressPath{
											{
												Path: "/bar/path",
												Backend: extensions.IngressBackend{
													ServiceName: "bar",
													ServicePort: intstr.FromInt(9000),
												},
											},
											{
												Path: "/baz/path",
												Backend: extensions.IngressBackend{
													ServiceName: "baz",
													ServicePort: intstr.FromInt(9001),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			changed: true,
			expected: `
global
	daemon
	maxconn 10000
	pidfile /var/run/haproxy.pid
	log /dev/log local5
	log 127.0.0.1 local0
	tune.bufsize 16384
	tune.maxrewrite 1024
	spread-checks 4

defaults
	log global
	mode http
	timeout connect 15s
	timeout client 60s
	timeout server 150s
	timeout queue 60s
	timeout http-request 15s
	timeout http-keep-alive 15s
	option httplog
	option redispatch
	option dontlognull

listen stats
	bind 127.0.0.1:3000
	mode http
	stats enable
	stats uri /

resolvers dns
	hold valid 10s

backend not_found
	# This seems abusive.
	errorfile 503 /etc/haproxy/errors/not_found.http

frontend ingress
	bind :80

	# Order matters for below log-format.
	capture request header User-Agent len 128
	capture request header Host len 64

	# JSON logging for ES: http://www.rsyslog.com/json-elasticsearch/
	log-format @cee:{"program":"haproxy","timestamp":%Ts,"http_status":%ST,"http_request":"%r","remote_addr":"%ci","bytes_read":%B,"upstream_addr":"%si","backend_name":"%b","retries":%rc,"bytes_uploaded":%U,"upstream_response_time":"%Tr","upstream_connect_time":"%Tc","session_duration":"%Tt","termination_state":"%ts","user_agent":"%[capture.req.hdr(1),json("utf8s")]","request_host":"%[capture.req.hdr(2),json("utf8s")]","host":"hostname"}

	# Host ACLs

	acl is_default_foo hdr_beg(host) -i foo.example.com
	acl is_default_bar hdr_beg(host) -i bar.example.com

	# Path ACLs and use_backend

	acl is_default_foo_path path_beg /
	use_backend default_foo if is_default_foo is_default_foo_path
	acl is_default_bar_bar_path_path path_beg /bar/path
	use_backend default_bar_bar_path if is_default_bar is_default_bar_bar_path_path
	acl is_default_bar_baz_path_path path_beg /baz/path
	use_backend default_bar_baz_path if is_default_bar is_default_bar_baz_path_path

	default_backend not_found



backend default_foo
	# Close connections after the proxy.
	option http-server-close
	# Include X-Forward-For header.
	option forwardfor

	balance leastconn
	server foo foo.default.svc.cluster.local:3000 resolvers dns
backend default_bar_bar_path
	# Close connections after the proxy.
	option http-server-close
	# Include X-Forward-For header.
	option forwardfor

	balance leastconn
	server bar bar.default.svc.cluster.local:9000 resolvers dns
backend default_bar_baz_path
	# Close connections after the proxy.
	option http-server-close
	# Include X-Forward-For header.
	option forwardfor

	balance leastconn
	server bar baz.default.svc.cluster.local:9001 resolvers dns
`,
		},
	}

	for _, test := range tests {
		dir, cleanup := testDir(t)
		defer cleanup()

		confPath := dir + "/file"
		c := NewConfig(&fakeIngress{listResults: test.ingresses}, "hostname", confPath, "example.com")

		changed, err := c.Update()
		if err != test.err {
			t.Fatalf("unexpected error: %v", err)
		}

		if changed != test.changed {
			t.Logf("want: %v", test.changed)
			t.Logf(" got: %v", changed)
			t.Fatalf("unexpected change value")
		}

		contents, err := ioutil.ReadFile(confPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if string(contents) != test.expected {
			ef, err := ioutil.TempFile("", "expected")
			if err != nil {
				panic(err)
			}
			_, err = ef.Write([]byte(test.expected))
			if err != nil {
				panic(err)
			}
			ef.Close()
			defer os.Remove(ef.Name())

			cf, err := ioutil.TempFile("", "contents")
			if err != nil {
				panic(err)
			}
			_, err = cf.Write([]byte(contents))
			if err != nil {
				panic(err)
			}
			cf.Close()
			defer os.Remove(cf.Name())

			output, _ := exec.Command("diff", ef.Name(), cf.Name()).CombinedOutput()

			t.Logf("diff results:\n%s", string(output))
			t.Fatal("unexpected config contents")
		}

		if updated, err := c.Update(); updated != false && err != nil {
			t.Fatal("update should not update with duplicate calls")
		}
	}

}

type fakeIngress struct {
	testclient.FakeIngress
	listResults []extensions.Ingress
}

func (f *fakeIngress) List(lo api.ListOptions) (*extensions.IngressList, error) {
	return &extensions.IngressList{Items: f.listResults}, nil
}
