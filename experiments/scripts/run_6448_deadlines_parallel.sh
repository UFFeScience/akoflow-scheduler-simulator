#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
repetitions="${EXPERIMENT_REPETITIONS:-30}"
workers="${EXPERIMENT_WORKERS_PER_DEADLINE:-2}"
scenarios="cluster_homo,cluster_hetero,cloud_homo,cloud_hetero,hybrid_homo,hybrid_hetero,hybrid_raspberry_500mbps"
prefix="${EXPERIMENT_PREFIX:-prism-latest}"

run_case() {
  margin_name="$1"
  deadline_margin="$2"
  output_name="${prefix}-all-environments-500mbps-vs-heft-colocation-montage-6448-deadline-${margin_name}-exp-01"
  if [ -s "${repo_root}/experiments/results/${output_name}/raw_results.csv" ]; then
    return
  fi
  docker compose run --rm --no-deps \
    -v "${repo_root}/experiments/results:/results" \
    backend go run . \
    -experiment-output="/results/${output_name}" \
    -experiment-repetitions="${repetitions}" \
    -experiment-workers="${workers}" \
    -experiment-beam-width=120 \
    -experiment-recommendations=100 \
    -experiment-prism-priority=adaptive_ready \
    -experiment-heft-mode=colocation \
    -experiment-workflow=montage_dss_20d \
    -experiment-scenarios="${scenarios}" \
    -experiment-budget-margin=1.2 \
    -experiment-deadline-margin="${deadline_margin}"
}

run_case plus-20 1.2 &
pid_plus=$!
run_case minus-5 0.95 &
pid_minus_5=$!
run_case minus-10 0.90 &
pid_minus_10=$!
run_case minus-20 0.80 &
pid_minus_20=$!

wait "${pid_plus}"
wait "${pid_minus_5}"
wait "${pid_minus_10}"
wait "${pid_minus_20}"

for margin_name in plus-20 minus-5 minus-10 minus-20; do
  output_name="${prefix}-all-environments-500mbps-vs-heft-colocation-montage-6448-deadline-${margin_name}-exp-01"
  docker run --rm \
    --user "$(id -u):$(id -g)" \
    -e MPLCONFIGDIR=/tmp/matplotlib \
    -e EXPERIMENT_RESULT_DIR="${output_name}" \
    -v "${repo_root}:/workspace" \
    scheduler-simulator-notebooks \
    python /workspace/experiments/notebooks/plot_experiments.py
done
