package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/macb/hing/config"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
)

func reloadHaproxy(config, pidfile string) {
	pid, err := ioutil.ReadFile(pidfile)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("error reading pidfile: %v", err)
	}

	args := []string{"-f", config, "-p", pidfile}
	if string(pid) != "" {
		go reapProcess(string(pid))
		args = append(args, "-sf", string(pid))
	}

	out, err := exec.Command("haproxy", args...).CombinedOutput()
	if err != nil && err.Error() != "wait: no child processes" {
		log.Printf("ran command: %s", strings.Join(append([]string{"haproxy"}, args...), " "))
		log.Printf("output when restarting:\n%s", string(out))
		log.Fatalf("failed to reload haproxy: %v", err)
	}

	waitForChange(string(pid), pidfile)
}

func waitForChange(oldPid, pidfile string) {
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	after := time.After(30 * time.Second)

	for {
		select {
		case <-after:
			log.Fatal("haproxy failed to change pid within 30s")
		case <-t.C:
			pid, err := ioutil.ReadFile(pidfile)
			if err != nil {
				continue
			}

			if string(pid) != oldPid {
				return
			}
		}
	}
}

func reapProcess(spid string) {
	pid, err := strconv.Atoi(strings.TrimSuffix(spid, "\n"))
	if err != nil {
		log.Fatalf("failed to parse pid %s: %v", spid, err)
	}

	log.Printf("reaping process %d", pid)
	for {
		p, err := syscall.Wait4(pid, nil, 0, nil)

		if err != nil {
			if err == syscall.ECHILD {
				break
			}
			log.Fatalf("unexpected error when waiting: %v", err)
		}

		switch p {
		case 0:
			// There are more PIDs to reap.
			log.Print("waiting to reap more processes")
		case -1:
			log.Fatalf("unexpected pid value when waiting: %d", p)
		default:
			log.Printf("reaped process %d", p)
			return
		}
	}
}

func main() {
	path := "/etc/haproxy/haproxy.cfg"
	pidfile := "/var/run/haproxy.pid"
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

	reloadHaproxy(path, pidfile)

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
			reloadHaproxy(path, pidfile)
		} else {
			log.Print("haproxy config unchanged")
		}
	}
}
