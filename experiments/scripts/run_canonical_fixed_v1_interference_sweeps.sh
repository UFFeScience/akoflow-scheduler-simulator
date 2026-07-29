agi#!/bin/sh
set -eu

prefix="${EXPERIMENT_PREFIX:-prism-canonical-fixed-v1}"
repetitions="${EXPERIMENT_REPETITIONS:-30}"
workers="${EXPERIMENT_WORKERS:-6}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
recommendations="${EXPERIMENT_RECOMMENDATIONS:-100}"
repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"

run_protocol() {
  output_name="$1"
  workflow="$2"
  rate="$3"
  budget="${4:-0}"
  deadline="${5:-0}"

  if [ -s "${repo_root}/experiments/results/${output_name}/raw_results.csv" ]; then
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
    -experiment-prism-priority=adaptive_ready \
    -experiment-heft-mode=colocation \
    -experiment-workflow="${workflow}" \
    -experiment-scenarios=hybrid_hetero \
    -experiment-interference-rate="${rate}" \
    -experiment-budget-limit="${budget}" \
    -experiment-deadline-limit="${deadline}"
}

run_sweep() {
  workflow="$1"
  task_label="$2"
  sweep_name="${prefix}-interference-sweep-heft-colocation-montage-${task_label}-exp-01"
  sweep_dir="${repo_root}/experiments/results/${sweep_name}"
  calibration_name="${sweep_name}-calibration-zero"

  run_protocol "${calibration_name}" "${workflow}" 0

  budget="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["scenario_slas"]["hybrid_hetero"]["budget_limit"])' \
    "${repo_root}/experiments/results/${calibration_name}/manifest.json")"
  deadline="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["scenario_slas"]["hybrid_hetero"]["deadline_limit"])' \
    "${repo_root}/experiments/results/${calibration_name}/manifest.json")"

  mkdir -p "${sweep_dir}"
  for percent in 10 20 30 40 50 60 70 80 90; do
    rate="$(python3 -c 'import sys; print(int(sys.argv[1]) / 100)' "${percent}")"
    run_protocol "${sweep_name}/rate-${percent}" "${workflow}" "${rate}" "${budget}" "${deadline}"
  done

  docker run --rm \
    --user "$(id -u):$(id -g)" \
    -e MPLCONFIGDIR=/tmp/matplotlib \
    -v "${repo_root}:/workspace" \
    scheduler-simulator-notebooks \
    python /workspace/experiments/notebooks/plot_interference_sweep.py \
    "/workspace/experiments/results/${sweep_name}"
}

run_sweep montage_050d 58
run_sweep montage_dss_20d 6448
