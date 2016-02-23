package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"text/template"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
)

func shellout(cmd string) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Fatalf("failed to execute %v: %v, err: %v", cmd, string(out), err)
	}
}

func main() {
	path := "/etc/haproxy/haproxy.cfg"
	var ingclient client.IngressInterface
	if kubeclient, err := client.NewInCluster(); err != nil {
		log.Fatalf("failed to create client: %v.", err)
	} else {
		ingclient = kubeclient.Extensions().Ingress(api.NamespaceAll)
	}
	tmpl, _ := template.New("haproxy").Parse(fmt.Sprintf(haproxyconf, os.Getenv("BASE_HOSTNAME")))

	ratelimiter := util.NewTokenBucketRateLimiter(0.1, 1)
	known := &extensions.IngressList{}

	err := createConfig(tmpl, nil, path)
	if err != nil {
		log.Fatalf("failed to create initial config: %v", err)
	}

	// controller loop
	shellout("haproxy -f " + path)
	for {
		log.Print("retrieving ingresses.")
		ratelimiter.Accept()
		ingresses, err := ingclient.List(api.ListOptions{})
		if err != nil {
			log.Printf("error retrieving ingresses: %v", err)
			continue
		}
		if reflect.DeepEqual(ingresses.Items, known.Items) {
			log.Print("no new ingresses.")
			continue
		}

		known = ingresses
		err = createConfig(tmpl, ingresses, path)
		if err != nil {
			log.Fatalf("failed to open %v: %v", haproxyconf, err)

		}

		shellout(fmt.Sprintf("haproxy -f %s -p /var/run/haproxy.pid -sf $(cat /var/run/haproxy.pid)", path))
	}
}

func createConfig(tmpl *template.Template, ingresses *extensions.IngressList, path string) error {
	w, err := os.Create(path)
	if err != nil {
		return err
	}

	if ingresses == nil {
		ingresses = &extensions.IngressList{}
	}

	err = tmpl.Execute(w, ingresses)
	if err != nil {
		return err
	}

	return nil
}
