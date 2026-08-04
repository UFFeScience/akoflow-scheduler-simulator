#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
workers="${EXPERIMENT_WORKERS:-6}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
prefix="${EXPERIMENT_PREFIX:-prism-wfcommons-file-latency-sweep-p1-variable-lookahead-v1}"
scenarios="network_hpc_local,network_hpc_multisite,network_cloud_multiregion,network_hpc_cloud,network_edge_cloud,network_fog_hpc_cloud,network_wfcommons_overlay"

run_case() {
  latency_ms="$1"
  workflow_id="$2"
  label="$3"
  output_name="${prefix}-latency-${latency_ms}ms-${label}-data1x-no-container-overhead-nointerference-seed1-vs-heft-colocation-deadline-plus-20-exp-01"
  if [ ! -s "${repo_root}/experiments/results/${output_name}/raw_results.csv" ]; then
    docker compose run --rm --no-deps \
      -v "${repo_root}/experiments/results:/results" \
      backend go run . \
      -experiment-output="/results/${output_name}" \
      -experiment-repetitions=1 \
      -experiment-workers="${workers}" \
      -experiment-beam-width="${beam_width}" \
      -experiment-recommendations=100 \
      -experiment-prism-priority=adaptive_ready \
      -experiment-heft-mode=colocation \
      -experiment-workflow="${workflow_id}" \
      -experiment-scenarios="${scenarios}" \
      -experiment-network-latency-ms="${latency_ms}" \
      -experiment-interference-rate=0 \
      -experiment-data-scale=1 \
      -experiment-budget-margin=1.2 \
      -experiment-deadline-margin=1.2 \
      -experiment-disable-container-overhead=true \
      -experiment-export-schedules=true
  fi
  if [ "$(find "${repo_root}/experiments/results/${output_name}/figures" -maxdepth 1 -name '*.png' 2>/dev/null | wc -l | tr -d ' ')" -lt 26 ]; then
    docker run --rm \
      --user "$(id -u):$(id -g)" \
      -e MPLCONFIGDIR=/tmp/matplotlib \
      -e EXPERIMENT_RESULT_DIR="${output_name}" \
      -v "${repo_root}:/workspace" \
      scheduler-simulator-notebooks \
      python /workspace/experiments/notebooks/plot_experiments.py
  fi
}

for latency_ms in 1 10 50 100; do
  run_case "$latency_ms" wfcommons_srasearch_104 srasearch-104
  run_case "$latency_ms" wfcommons_soykb_676 soykb-676
  run_case "$latency_ms" wfcommons_1000genome_902 1000genome-902
  run_case "$latency_ms" wfcommons_seismology_1101 seismic-cross-correlation-1101
  run_case "$latency_ms" wfcommons_epigenomics_1695 epigenomics-1695
  run_case "$latency_ms" montage_dss_20d montage-6448
  run_case "$latency_ms" wfcommons_cycles_6543 cycles-6543
done
