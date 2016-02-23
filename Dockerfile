FROM haproxy:1.6.3

RUN mkdir -p /etc/haproxy/errors
ADD not_found.http /etc/haproxy/errors/not_found.http

ADD hing /hing

CMD ["/hing"]
