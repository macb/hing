package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"github.com/macb/hing/config"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
)

func startHaproxy(config string) {
	_, err := exec.Command("haproxy", "-f", config).CombinedOutput()
	if err != nil {
		log.Fatalf("haproxy failed to start: %v", err)
	}
}

func reloadHaproxy(config string) {
	pid, err := ioutil.ReadFile("/var/run/haproxy.pid")
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("error reading pidfile: %v", err)
	}

	args := []string{"-f", config, "-p", "/var/run/haproxy.pid"}
	if string(pid) != "" {
		args = []string{"-sf", string(pid)}
	}

	_, err = exec.Command("haproxy", args...).CombinedOutput()
	if err != nil {
		log.Fatalf("haproxy failed to restart: %v", err)
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

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("failed to get hostname: %v.", err)
	}
	c := config.NewConfig(ingclient, hostname, path, os.Getenv("BASE_DOMAIN"))
	_, err = c.Update()
	if err != nil {
		log.Fatalf("failed to create conf: %v", err)
	}

	go startHaproxy(path)

	// controller loop
	ratelimiter := util.NewTokenBucketRateLimiter(0.1, 1)
	for {
		ratelimiter.Accept()
		changed, err := c.Update()
		if err != nil {
			switch err.(type) {
			case config.ListError:
				log.Printf("failed to list ingresses: %s", err.Error())
				continue
			default:
				log.Fatalf("failed to update file: %v", err)
			}
		}

		if changed {
			log.Print("reloading haproxy")
			go reloadHaproxy(path)
		} else {
			log.Print("haproxy config unchanged")
		}
	}
}
