package main

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

{{ range $ing := .Items }}{{ range $rule := $ing.Spec.Rules }}
	acl is_{{$ing.Namespace}}_{{$rule.Host}} hdr_beg(host) -i {{$rule.Host}}.%s
{{ range $idx, $path := $rule.HTTP.Paths }}
	acl is_{{$ing.Namespace}}_{{$ing.Name}}_{{$idx}} path_beg -i {{$path.Path}}
	use_backend {{$rule.Host}}_{{$idx}} if is_{{$ing.Namespace}}_{{$rule.Host}} is_{{$ing.Namespace}}_{{$ing.Name}}_{{$idx}}
{{end}}{{end}}{{end}}
	default_backend not_found


{{ range $ing := .Items }}
{{ range $rule := $ing.Spec.Rules }}
{{ range $idx, $path := $rule.HTTP.Paths }}
backend {{$rule.Host}}_{{$idx}}
	balance leastconn
	server {{$rule.Host}} {{$path.Backend.ServiceName}}.{{$ing.Namespace}}.svc.cluster.local:{{$path.Backend.ServicePort}} resolvers dns
{{end}}{{end}}{{end}}
`
