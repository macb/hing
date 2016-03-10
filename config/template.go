package config

const haproxyconf = `
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
	log-format @cee:{"program":"haproxy","timestamp":%Ts,"http_status":%ST,"http_request":"%r","remote_addr":"%ci","bytes_read":%B,"upstream_addr":"%si","backend_name":"%b","retries":%rc,"bytes_uploaded":%U,"upstream_response_time":"%Tr","upstream_connect_time":"%Tc","session_duration":"%Tt","termination_state":"%ts","user_agent":"%[capture.req.hdr(1),json]","host":"%[capture.req.hdr(2),json]"}

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
	# Close connections after the proxy.
	option http-server-close
	# Include X-Forward-For header.
	option forwardfor

	balance leastconn
	server {{$be.Server}} resolvers dns{{end}}
`
