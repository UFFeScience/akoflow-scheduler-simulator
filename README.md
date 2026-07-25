# Scheduler Simulator

Greenfield Docker Compose app for simulating Akoflow-style scientific workflow scheduling.

## Beam Search Recommendation Animation

![Beam search scheduling recommendation animation](doc/assets/beam-search-animation.gif)

The animation shows how the scheduler expands candidate schedules, applies beam pruning, ranks final schedules, and selects `Recommendation #1`.

## Run

```bash
docker compose up --build
```

- Frontend: http://localhost:5173
- Backend: http://localhost:8000
- API schema: http://localhost:8000/api/schema

The compose stack is development-only. The backend starts through Delve so you
can attach your IDE debugger to `localhost:2345`.

For VS Code debugging, use **Dev Containers: Reopen in Container** and run the
`Debug Backend (Dev Container)` launch configuration. This keeps Go and Delve
inside Docker, so the host machine does not need a local Go installation.

## Backend Tests Through Docker Compose

```bash
docker compose run --rm backend go test ./...
```

## Montage Experimental Protocol

The backend includes a reproducible batch runner that compares Beam-Time
(`100% time`), Beam-Cost (`100% cost`), and HEFT across the six machine
scenarios in `experiments/machine_simulators.csv`.
It imports the 58-task Montage DAG and runs the paired interference seeds. HEFT
is executed first for every scenario and seed. The global mean HEFT makespan
becomes the fixed deadline, and the global mean HEFT cost becomes the fixed
budget for every execution, without any margin.

Machine prices are read from `experiments/machine_simulators.csv` in USD per
machine-hour. Google Cloud entries use on-demand `us-central1` prices. PlaFRIM
entries intentionally use `0 USD/hour` under the `plafrim_no_direct_charge`
model because they do not generate a cloud bill. Network prices are stored in
USD/GB and converted to USD/MB.

Final schedule cost bills each cloud VM at its full hourly price from its first
boot/start until its last assigned task finishes. This matches the reference
calculation for the original C3D VM. PlaFRIM active intervals remain free.

Run the complete protocol:

```bash
mkdir -p experiments/results
docker compose run --rm --no-deps \
  -v "${PWD}/experiments/results:/results" \
  backend \
  go run . \
  -experiment-output /results \
  -experiment-repetitions 30 \
  -experiment-beam-width 120
```

The output directory contains:

- `manifest.json`: fixed seeds, global HEFT mean limits, algorithms, and scenarios;
- `raw_results.csv`: one row per algorithm, scenario, and interference seed;
- `summary.csv`: mean, median, standard deviation, 95% confidence interval,
  feasibility ratio, and Beam gains over HEFT.

For a quick validation, set `-experiment-repetitions 1`. The structural seed is
fixed at 42; interference seeds run from 1 through the requested repetition
count.

### Generate the experiment charts with Jupyter in Docker

The notebooks validate `raw_results.csv` and generate the complete chart set:

```bash
sh experiments/notebooks/run-with-docker.sh
```

The script builds the `scheduler-simulator-notebooks` image and executes both
notebooks through `docker run --rm`. No host Python installation is required.

The executed notebooks are stored in `experiments/notebooks`, and the generated
PNGs plus their Portuguese descriptions are written to
`experiments/results/figures`.
# akoflow-scheduler-simulator
