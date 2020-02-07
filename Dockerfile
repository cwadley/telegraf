# FROM golang:1.11.0 as builder
# COPY . /build
# WORKDIR /build
# RUN make deps && make static

FROM docker.teledev.io/baseimages/bashbase:latest
COPY ./telegraf /usr/local/bin

RUN mkdir /etc/telegraf \
    && /usr/local/bin/telegraf config > /etc/telegraf/telegraf.conf

CMD ["telegraf"]
