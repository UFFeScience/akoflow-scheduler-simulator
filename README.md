# Scheduler Simulator

Greenfield Docker Compose app for simulating Akoflow-style scientific workflow scheduling.

## Run

```bash
docker compose up --build
```

- Frontend: http://localhost:5173
- Backend: http://localhost:8000
- API schema: http://localhost:8000/api/schema

## Backend Tests Through Docker Compose

```bash
docker compose run --rm backend pytest
```
