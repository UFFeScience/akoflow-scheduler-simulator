#!/bin/sh
set -eu

prefix="${EXPERIMENT_PREFIX:-prism-canonical-fixed-v1}"
repetitions="${EXPERIMENT_REPETITIONS:-30}"
workers="${EXPERIMENT_WORKERS:-6}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
recommendations="${EXPERIMENT_RECOMMENDATIONS:-100}"
priority="${EXPERIMENT_PRIORITY:-adaptive_ready}"
repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"

run_experiment() {
  heft_mode="$1"
  workflow="$2"
  task_label="$3"
  heft_label="$4"
  output_name="${prefix}-vs-heft-${heft_label}-montage-${task_label}-exp-01"

  if [ ! -s "${repo_root}/experiments/results/${output_name}/raw_results.csv" ]; then
    docker compose run --rm --no-deps \
      -v "${repo_root}/experiments/results:/results" \
      backend go run . \
      -experiment-output="/results/${output_name}" \
      -experiment-repetitions="${repetitions}" \
      -experiment-workers="${workers}" \
      -experiment-beam-width="${beam_width}" \
      -experiment-recommendations="${recommendations}" \
      -experiment-prism-priority="${priority}" \
      -experiment-heft-mode="${heft_mode}" \
      -experiment-workflow="${workflow}"
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

run_experiment classic_no_colocation montage_050d 58 classic
run_experiment colocation montage_050d 58 colocation
run_experiment classic_no_colocation montage_dss_20d 6448 classic
run_experiment colocation montage_dss_20d 6448 colocation
