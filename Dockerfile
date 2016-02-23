FROM haproxy:1.6.3

RUN mkdir /etc/haproxy

ADD hing /hing

CMD ["/hing"]
