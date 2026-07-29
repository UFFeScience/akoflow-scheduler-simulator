#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
sweep_name="${EXPERIMENT_OUTPUT_NAME:-prism-edge-cloud-extreme-interference-sweep-heft-colocation-montage-6448-exp-01}"
repetitions="${EXPERIMENT_REPETITIONS:-30}"
workers="${EXPERIMENT_WORKERS:-6}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
recommendations="${EXPERIMENT_RECOMMENDATIONS:-100}"
calibration_name="${sweep_name}-calibration-zero"

run_rate() {
  output_name="$1"
  rate="$2"
  budget="$3"
  deadline="$4"
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
    -experiment-workflow=montage_dss_20d \
    -experiment-scenarios=edge_cloud_extreme \
    -experiment-interference-rate="${rate}" \
    -experiment-budget-limit="${budget}" \
    -experiment-deadline-limit="${deadline}"
}

run_rate "${calibration_name}" 0 0 0
budget="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["scenario_slas"]["edge_cloud_extreme"]["budget_limit"])' \
  "${repo_root}/experiments/results/${calibration_name}/manifest.json")"
deadline="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["scenario_slas"]["edge_cloud_extreme"]["deadline_limit"])' \
  "${repo_root}/experiments/results/${calibration_name}/manifest.json")"

mkdir -p "${repo_root}/experiments/results/${sweep_name}"
for percent in 10 20 30 40 50 60 70 80 90; do
  rate="$(python3 -c 'import sys; print(int(sys.argv[1]) / 100)' "${percent}")"
  run_rate "${sweep_name}/rate-${percent}" "${rate}" "${budget}" "${deadline}"
done

docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e MPLCONFIGDIR=/tmp/matplotlib \
  -v "${repo_root}:/workspace" \
  scheduler-simulator-notebooks \
  python /workspace/experiments/notebooks/plot_interference_sweep.py \
  "/workspace/experiments/results/${sweep_name}"
