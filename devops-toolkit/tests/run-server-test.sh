#!/bin/bash
set -e
echo "运行 Docker 容器测试..."
docker run -it --rm -v $PWD:/test alpine sh -c "$PWD/tests/server.test.js"

