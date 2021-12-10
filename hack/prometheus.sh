#!/bin/bash

podman run --net host --rm -it -v ./prometheus.yml:/etc/prometheus/prometheus.yml:Z -p 9090:9090 quay.io/prometheus/prometheus
