from __future__ import annotations

from typing import Dict

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware

from app.models import GeneratedSimulation, ScheduleOptimizationResponse, SimulationRequest, SimulationResult
from app.services.generation_services import GenerateSimulationService, PRESETS
from app.services.scheduling_services import BeamScheduleOptimizerService, ScheduleWorkflowService

app = FastAPI(title="Scheduler Simulator API", version="0.1.0")
app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:5173", "http://127.0.0.1:5173"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

simulations: Dict[str, SimulationResult] = {}
generator = GenerateSimulationService()
scheduler = ScheduleWorkflowService()
optimizer = BeamScheduleOptimizerService()


@app.get("/health")
def health() -> Dict[str, str]:
    return {"status": "ok"}


@app.get("/api/presets")
def presets() -> Dict[str, object]:
    return {"presets": PRESETS}


@app.get("/api/schema")
def schema() -> Dict[str, object]:
    return app.openapi()


@app.post("/api/simulations/generate-only", response_model=GeneratedSimulation)
def generate_only(request: SimulationRequest) -> GeneratedSimulation:
    return generator.execute(request)


@app.post("/api/simulations/run", response_model=SimulationResult)
def run_simulation(request: SimulationRequest) -> SimulationResult:
    try:
        result = scheduler.execute(generator.execute(request))
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    simulations[result.id] = result
    return result


@app.post("/api/simulations/schedule", response_model=ScheduleOptimizationResponse)
def schedule_generated_simulation(generated: GeneratedSimulation) -> ScheduleOptimizationResponse:
    try:
        response = optimizer.execute(generated)
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    for option in response.options:
        simulations[option.result.id] = option.result
    return response
