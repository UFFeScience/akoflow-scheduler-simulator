#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
repetitions="${EXPERIMENT_REPETITIONS:-1}"
workers="${EXPERIMENT_WORKERS:-6}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
recommendations="${EXPERIMENT_RECOMMENDATIONS:-100}"
prefix="${EXPERIMENT_PREFIX:-prism-network-critical-7env}"
scenarios="network_hpc_local,network_hpc_multisite,network_cloud_multiregion,network_hpc_cloud,network_edge_cloud,network_fog_hpc_cloud,network_wfcommons_overlay"

run_case() {
  workflow="$1"
  workflow_label="$2"
  data_scale="$3"
  output_name="${prefix}-${workflow_label}-data${data_scale}x-no-container-overhead-nointerference-seed1-v1-vs-heft-colocation-deadline-plus-20-exp-01"

  if [ ! -s "${repo_root}/experiments/results/${output_name}/raw_results.csv" ]; then
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
      -experiment-workflow="${workflow}" \
      -experiment-scenarios="${scenarios}" \
      -experiment-interference-rate=0 \
      -experiment-data-scale="${data_scale}" \
      -experiment-budget-margin=1.2 \
      -experiment-deadline-margin=1.2 \
      -experiment-disable-container-overhead=true \
      -experiment-export-schedules=true
  else
    echo "Skipping completed experiment: ${output_name}"
  fi

  docker run --rm \
    --user "$(id -u):$(id -g)" \
    -e MPLCONFIGDIR=/tmp/matplotlib \
    -e EXPERIMENT_RESULT_DIR="${output_name}" \
    -v "${repo_root}:/workspace" \
    scheduler-simulator-notebooks \
    python /workspace/experiments/notebooks/plot_experiments.py
}

for data_scale in 1 10 100; do
  run_case montage_050d montage-58 "${data_scale}"
  run_case montage_dss_20d montage-6448 "${data_scale}"
  run_case image_dataflow_8 image-dataflow-8 "${data_scale}"
done
