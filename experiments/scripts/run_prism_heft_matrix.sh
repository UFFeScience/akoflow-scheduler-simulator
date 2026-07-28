#!/bin/sh
set -eu

repetitions="${EXPERIMENT_REPETITIONS:-30}"
workers="${EXPERIMENT_WORKERS:-4}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
recommendations="${EXPERIMENT_RECOMMENDATIONS:-100}"
repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"

run_experiment() {
  priority="$1"
  heft_mode="$2"
  workflow="$3"
  output_name="$4"

  if [ -s "${repo_root}/experiments/results/${output_name}/raw_results.csv" ] &&
    [ -s "${repo_root}/experiments/results/${output_name}/summary.csv" ] &&
    [ -s "${repo_root}/experiments/results/${output_name}/manifest.json" ]; then
    echo "Skipping completed experiment: ${output_name}"
    return
  fi

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
}

run_experiment topological_order colocation montage_050d \
  prism-topology-vs-heft-colocation-montage-58-exp-01
run_experiment topological_order classic_no_colocation montage_050d \
  prism-topology-vs-heft-classic-montage-58-exp-01
run_experiment upward_rank classic_no_colocation montage_050d \
  prism-uprank-vs-heft-classic-montage-58-exp-01
run_experiment upward_rank colocation montage_050d \
  prism-uprank-vs-heft-colocation-montage-58-exp-01

run_experiment topological_order colocation montage_dss_20d \
  prism-topology-vs-heft-colocation-montage-6448-exp-01
run_experiment topological_order classic_no_colocation montage_dss_20d \
  prism-topology-vs-heft-classic-montage-6448-exp-01
run_experiment upward_rank classic_no_colocation montage_dss_20d \
  prism-uprank-vs-heft-classic-montage-6448-exp-01
run_experiment upward_rank colocation montage_dss_20d \
  prism-uprank-vs-heft-colocation-montage-6448-exp-01
