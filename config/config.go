package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/template"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

var (
	tmpl = template.Must(template.New("haproxy").Parse(haproxyconf))
)

type ListError struct {
	e error
}

func (l ListError) Error() string {
	return l.e.Error()
}

type Config struct {
	path, baseDomain string
	client           unversioned.IngressInterface

	previous *extensions.IngressList
}

func NewConfig(client unversioned.IngressInterface, path, baseDomain string) *Config {
	return &Config{
		path:       path,
		client:     client,
		baseDomain: baseDomain,
		previous:   &extensions.IngressList{},
	}
}

type backend struct {
	Name   string
	Server string
}

type frontend struct {
	HostACL acl
	PathACL acl
	Backend backend
}

type acl struct {
	Name, Matcher string
}

func (c Config) Update() error {
	l, err := c.client.List(api.ListOptions{})
	if err != nil {
		return ListError{err}
	}

	if reflect.DeepEqual(l.Items, c.previous.Items) {
		return nil
	}

	backends, hostACLs, frontends := featuresFrom(l.Items, c.baseDomain)

	data := struct {
		Backends  []backend
		Frontends []frontend
		HostACLs  []acl
	}{
		Backends:  backends,
		Frontends: frontends,
		HostACLs:  hostACLs,
	}

	w, err := os.Create(c.path)
	if err != nil {
		return err
	}
	defer w.Close()

	err = tmpl.Execute(w, data)
	if err != nil {
		return err
	}

	c.previous = l
	return nil
}

func featuresFrom(ingresses []extensions.Ingress, baseDomain string) (backends []backend, hostACLs []acl, frontends []frontend) {
	for _, i := range ingresses {
		for _, rule := range i.Spec.Rules {
			hostACL := acl{
				Name:    fmt.Sprintf("is_%s_%s", i.Namespace, rule.Host),
				Matcher: fmt.Sprintf("hdr_beg(host) -i %s", rule.Host+"."+baseDomain),
			}

			hostACLs = append(hostACLs, hostACL)

			for _, path := range rule.HTTP.Paths {
				name := canonicalizedName(i.Namespace, rule.Host, path.Path)

				b := backend{
					Name:   name,
					Server: fmt.Sprintf("%s %s.%s.svc.cluster.local:%s", rule.Host, path.Backend.ServiceName, i.Namespace, path.Backend.ServicePort.String()),
				}
				backends = append(backends, b)

				pathACL := acl{
					Name:    fmt.Sprintf("is_%s", name),
					Matcher: fmt.Sprintf("hdr_beg(host) -i %s", rule.Host),
				}

				frontends = append(frontends, frontend{
					HostACL: hostACL,
					PathACL: pathACL,
					Backend: b,
				})

			}
		}
	}

	return backends, hostACLs, frontends
}

func canonicalizedName(namespace, host, path string) string {
	// Replace / in the path with _ and trim any _ prefixes
	cPath := strings.Replace(path, "/", "_", -1)
	cPath = strings.TrimLeft(cPath, "_")

	// Replace - and . in hostnames with _
	cHost := strings.Replace(host, "-", "_", -1)
	cHost = strings.Replace(cHost, ".", "_", -1)

	// Trim all trailing _ and lowercase
	return strings.ToLower(strings.TrimRight(fmt.Sprintf("%s_%s_%s", namespace, cHost, cPath), "_"))
}
