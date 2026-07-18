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
# akoflow-scheduler-simulator
