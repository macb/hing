package config

const haproxyconf = `
global
	daemon
	maxconn 10000
	pidfile /var/run/haproxy.pid
	# log /dev/log local5
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
	balance source

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

	# Host ACLs
{{ range $acl := .HostACLs }}
	acl {{$acl.Name}} {{$acl.Matcher}}{{end}}

	# Path ACLs and use_backend
{{ range $fe := .Frontends }}
	acl {{$fe.PathACL.Name}} {{$fe.PathACL.Matcher}}
	use_backend {{$fe.Backend.Name}} if {{$fe.HostACL.Name}} {{$fe.PathACL.Name}}{{end}}

	default_backend not_found


{{ range $be := .Backends }}
backend {{$be.Name}}
	balance leastconn
	server {{$be.Server}} resolvers dns{{end}}
`
