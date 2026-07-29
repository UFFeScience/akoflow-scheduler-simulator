#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
output_name="${EXPERIMENT_OUTPUT_NAME:-prism-edge-cloud-extreme-vs-heft-colocation-montage-6448-exp-01}"
repetitions="${EXPERIMENT_REPETITIONS:-30}"
workers="${EXPERIMENT_WORKERS:-6}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
recommendations="${EXPERIMENT_RECOMMENDATIONS:-100}"

docker compose run --rm --no-deps \
  -v "${repo_root}/experiments/results:/results" \
  backend go run . \
  -experiment-output="/results/${output_name}" \
  -experiment-repetitions="${repetitions}" \
  -experiment-workers="${workers}" \
  -experiment-beam-width="${beam_width}" \
  -experiment-recommendations="${recommendations}" \
  -experiment-prism-priority=adaptive_ready \
  -experiment-heft-mode=colocation \
  -experiment-workflow=montage_dss_20d \
  -experiment-scenarios=edge_cloud_extreme
