#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
matrix_root="${MATRIX_ROOT:-latency-datax-envs-workflows-complete-v1}"
workers="${EXPERIMENT_WORKERS:-6}"
beam_width="${EXPERIMENT_BEAM_WIDTH:-120}"
scenarios="network_hpc_local,network_hpc_multisite,network_cloud_multiregion,network_hpc_cloud,network_edge_cloud,network_fog_hpc_cloud,network_wfcommons_overlay"

run_case() {
  latency_ms="$1" bandwidth_mbps="$2" data_scale="$3" sla_margin="$4" sla_label="$5" workflow_id="$6" workflow_label="$7"
  profile="latency-${latency_ms}ms-bandwidth-${bandwidth_mbps}mbps"
  output_name="${matrix_root}/${profile}/data-${data_scale}x/sla-${sla_label}/${workflow_label}"
  output_dir="${repo_root}/experiments/results/${output_name}"
  if [ ! -s "${output_dir}/raw_results.csv" ]; then
    mkdir -p "${output_dir}"
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
      -experiment-network-bandwidth-mbps="${bandwidth_mbps}" \
      -experiment-interference-rate=0 \
      -experiment-data-scale="${data_scale}" \
      -experiment-budget-margin="${sla_margin}" \
      -experiment-deadline-margin="${sla_margin}" \
      -experiment-disable-container-overhead=true \
      -experiment-export-schedules=false
  fi
  figure_count="$(find "${output_dir}/figures" -maxdepth 1 -name '*.png' 2>/dev/null | wc -l | tr -d ' ')"
  if [ "${figure_count}" -lt 26 ]; then
    docker run --rm \
      --user "$(id -u):$(id -g)" \
      -e MPLCONFIGDIR=/tmp/matplotlib \
      -e EXPERIMENT_RESULT_DIR="${output_name}" \
      -v "${repo_root}:/workspace" \
      scheduler-simulator-notebooks \
      python /workspace/experiments/notebooks/plot_experiments.py
  fi
}

run_workflows() {
  latency_ms="$1" bandwidth_mbps="$2" data_scale="$3" sla_margin="$4" sla_label="$5"
  run_case "$latency_ms" "$bandwidth_mbps" "$data_scale" "$sla_margin" "$sla_label" wfcommons_srasearch_104 srasearch-104
  run_case "$latency_ms" "$bandwidth_mbps" "$data_scale" "$sla_margin" "$sla_label" wfcommons_soykb_676 soykb-676
  run_case "$latency_ms" "$bandwidth_mbps" "$data_scale" "$sla_margin" "$sla_label" wfcommons_1000genome_902 1000genome-902
  run_case "$latency_ms" "$bandwidth_mbps" "$data_scale" "$sla_margin" "$sla_label" wfcommons_seismology_1101 seismic-cross-correlation-1101
  run_case "$latency_ms" "$bandwidth_mbps" "$data_scale" "$sla_margin" "$sla_label" wfcommons_epigenomics_1695 epigenomics-1695
  run_case "$latency_ms" "$bandwidth_mbps" "$data_scale" "$sla_margin" "$sla_label" montage_dss_20d montage-6448
  run_case "$latency_ms" "$bandwidth_mbps" "$data_scale" "$sla_margin" "$sla_label" wfcommons_cycles_6543 cycles-6543
}

for network_profile in "100:500" "1000:100" "10000:10"; do
  latency_ms="${network_profile%%:*}"
  bandwidth_mbps="${network_profile##*:}"
  for data_scale in 1 10 100; do
    run_workflows "$latency_ms" "$bandwidth_mbps" "$data_scale" 1.20 plus-20
    run_workflows "$latency_ms" "$bandwidth_mbps" "$data_scale" 0.95 minus-5
    run_workflows "$latency_ms" "$bandwidth_mbps" "$data_scale" 0.90 minus-10
    run_workflows "$latency_ms" "$bandwidth_mbps" "$data_scale" 0.80 minus-20
  done
done

docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e MPLCONFIGDIR=/tmp/matplotlib \
  -e MATRIX_ROOT="${matrix_root}" \
  -v "${repo_root}:/workspace" \
  scheduler-simulator-notebooks \
  python /workspace/experiments/scripts/aggregate_complete_latency_data_sla_matrix.py

runtime_root="/Users/ovvesley/.cache/codex-runtimes/codex-primary-runtime/dependencies"
ln -sfn "${runtime_root}/node/node_modules" "${repo_root}/experiments/scripts/node_modules"
"${runtime_root}/node/bin/node" "${repo_root}/experiments/scripts/build_complete_matrix_workbook.mjs" "${matrix_root}"
rm -f "${repo_root}/experiments/scripts/node_modules"
