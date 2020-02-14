FROM golang:1.13.7 as builder
COPY . /build
WORKDIR /build
RUN make deps && make test && make static

FROM docker.teledev.io/baseimages/bashbase:latest
COPY --from=builder /build/telegraf /usr/local/bin

RUN mkdir /etc/telegraf \
    && /usr/local/bin/telegraf config > /etc/telegraf/telegraf.conf

CMD ["telegraf"]
