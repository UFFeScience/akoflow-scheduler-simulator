#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
repetitions="${EXPERIMENT_REPETITIONS:-1}"
workers="${EXPERIMENT_WORKERS:-6}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
recommendations="${EXPERIMENT_RECOMMENDATIONS:-100}"
prefix="${EXPERIMENT_PREFIX:-prism-image-dataflow-8-real-network-stress-data1x}"

run_rate() {
  scenario="$1"
  output_name="$2"
  rate="$3"
  budget="$4"
  deadline="$5"

  if [ -s "${repo_root}/experiments/results/${output_name}/raw_results.csv" ]; then
    echo "Skipping completed experiment: ${output_name}"
    return
  fi

  echo "Running ${scenario} at interference rate ${rate}: ${output_name}"
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
    -experiment-workflow=image_dataflow_8 \
    -experiment-scenarios="${scenario}" \
    -experiment-interference-rate="${rate}" \
    -experiment-data-scale=1 \
    -experiment-budget-margin=1.20 \
    -experiment-deadline-margin=1.20 \
    -experiment-budget-limit="${budget}" \
    -experiment-deadline-limit="${deadline}"
}

for scenario in \
  real_network_stress_cluster_hetero \
  real_network_stress_cloud_hetero \
  real_network_stress_hybrid_raspberry_500mbps
do
  case "${scenario}" in
    real_network_stress_cluster_hetero) environment_label="cluster-hetero" ;;
    real_network_stress_cloud_hetero) environment_label="cloud-hetero" ;;
    real_network_stress_hybrid_raspberry_500mbps) environment_label="hybrid-fog-hpc-cloud" ;;
  esac

  sweep_name="${prefix}-${environment_label}-interference-sweep-seed1-v1-vs-heft-colocation-exp-01"
  calibration_name="${sweep_name}-calibration-zero"

  run_rate "${scenario}" "${calibration_name}" 0 0 0
  budget="$(python3 -c 'import json,sys; data=json.load(open(sys.argv[1])); print(data["scenario_slas"][sys.argv[2]]["budget_limit"])' \
    "${repo_root}/experiments/results/${calibration_name}/manifest.json" "${scenario}")"
  budget="$(python3 -c 'import sys; value=float(sys.argv[1]); print(value if value > 0 else 1e-12)' "${budget}")"
  deadline="$(python3 -c 'import json,sys; data=json.load(open(sys.argv[1])); print(data["scenario_slas"][sys.argv[2]]["deadline_limit"])' \
    "${repo_root}/experiments/results/${calibration_name}/manifest.json" "${scenario}")"

  mkdir -p "${repo_root}/experiments/results/${sweep_name}"
  for percent in 10 20 30 40 50 60 70 80 90; do
    rate="$(python3 -c 'import sys; print(int(sys.argv[1]) / 100)' "${percent}")"
    run_rate "${scenario}" "${sweep_name}/rate-${percent}" "${rate}" "${budget}" "${deadline}"
  done

  docker run --rm \
    --user "$(id -u):$(id -g)" \
    -e MPLCONFIGDIR=/tmp/matplotlib \
    -v "${repo_root}:/workspace" \
    scheduler-simulator-notebooks \
    python /workspace/experiments/notebooks/plot_interference_sweep.py \
    "/workspace/experiments/results/${sweep_name}"
done
