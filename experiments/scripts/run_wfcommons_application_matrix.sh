#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
prefix="${EXPERIMENT_PREFIX:-prism-wfcommons-applications-file-latency-p1-variable-lookahead-v2}"
workers="${EXPERIMENT_WORKERS:-6}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
scenarios="network_hpc_local,network_hpc_multisite,network_cloud_multiregion,network_hpc_cloud,network_edge_cloud,network_fog_hpc_cloud,network_wfcommons_overlay"

run_case() {
  workflow_id="$1"
  label="$2"
  output_name="${prefix}-${label}-data1x-no-container-overhead-nointerference-seed1-vs-heft-colocation-deadline-plus-20-exp-01"
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
      -experiment-interference-rate=0 \
      -experiment-data-scale=1 \
      -experiment-budget-margin=1.2 \
      -experiment-deadline-margin=1.2 \
      -experiment-disable-container-overhead=true \
      -experiment-export-schedules=true
  fi
  docker run --rm \
    --user "$(id -u):$(id -g)" \
    -e MPLCONFIGDIR=/tmp/matplotlib \
    -e EXPERIMENT_RESULT_DIR="${output_name}" \
    -v "${repo_root}:/workspace" \
    scheduler-simulator-notebooks \
    python /workspace/experiments/notebooks/plot_experiments.py
}

run_case wfcommons_srasearch_104 srasearch-104
run_case wfcommons_soykb_676 soykb-676
run_case wfcommons_1000genome_902 1000genome-902
run_case wfcommons_seismology_1101 seismic-cross-correlation-1101
run_case wfcommons_epigenomics_1695 epigenomics-1695
run_case montage_dss_20d montage-6448
run_case wfcommons_cycles_6543 cycles-6543
