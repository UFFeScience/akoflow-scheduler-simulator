#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
repetitions="${EXPERIMENT_REPETITIONS:-30}"
workers="${EXPERIMENT_WORKERS:-6}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
recommendations="${EXPERIMENT_RECOMMENDATIONS:-100}"
scenarios="cluster_homo,cluster_hetero,cloud_homo,cloud_hetero,hybrid_homo,hybrid_hetero,hybrid_raspberry_500mbps"

run_margin() {
  margin_name="$1"
  deadline_margin="$2"
  output_name="prism-latest-all-environments-500mbps-vs-heft-colocation-montage-6448-deadline-${margin_name}-exp-01"

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
      -experiment-workflow=montage_dss_20d \
      -experiment-scenarios="${scenarios}" \
      -experiment-budget-margin=1.2 \
      -experiment-deadline-margin="${deadline_margin}"
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

run_margin plus-20 1.2
run_margin minus-20 0.8
