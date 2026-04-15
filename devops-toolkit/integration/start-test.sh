#!/bin/bash
set -e
docker-compose -f devices/docker/docker-compose.test.yml up -d
sleep 10
docker-compose -f devices/docker/docker-compose.test.yml ps
