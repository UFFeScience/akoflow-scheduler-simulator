#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)

docker build \
  -f "$repo_root/experiments/notebooks/Dockerfile" \
  -t scheduler-simulator-notebooks \
  "$repo_root"

docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e MPLCONFIGDIR=/tmp/matplotlib \
  -v "$repo_root:/workspace" \
  scheduler-simulator-notebooks
