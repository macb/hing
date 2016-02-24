package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/macb/hing/config"
	"k8s.io/kubernetes/pkg/api"
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

	c := config.NewConfig(ingclient, path, os.Getenv("BASE_DOMAIN"))
	err := c.Update()
	if err != nil {
		log.Fatalf("failed to create conf: %v", err)
	}

	shellout("haproxy -f " + path)

	// controller loop
	ratelimiter := util.NewTokenBucketRateLimiter(0.1, 1)
	for {
		ratelimiter.Accept()
		err := c.Update()
		if err != nil {
			log.Fatalf("failed to update file: %v", err)
		}

		shellout(fmt.Sprintf("haproxy -f %s -p /var/run/haproxy.pid -sf $(cat /var/run/haproxy.pid)", path))
	}
}
