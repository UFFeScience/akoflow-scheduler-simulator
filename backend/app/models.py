from __future__ import annotations

from typing import Dict, List, Literal, Optional, Tuple

from pydantic import BaseModel, Field


ResourceKind = Literal["cluster", "cloud"]
ResourceStatus = Literal["warm", "cold"]


class SimulationRequest(BaseModel):
    preset: str = "Montage"
    seed: int = 42
    task_count: int = Field(default=12, ge=3, le=80)
    edge_density: float = Field(default=0.22, ge=0.0, le=0.8)
    cluster_machines: int = Field(default=3, ge=1, le=20)
    cloud_machines: int = Field(default=2, ge=0, le=20)
    cores_per_machine: int = Field(default=2, ge=1, le=16)
    deadline: float = Field(default=160.0, gt=1.0)
    budget: float = Field(default=260.0, ge=0.0)
    weight_time: float = Field(default=0.55, ge=0.0, le=1.0)
    weight_cost: float = Field(default=0.30, ge=0.0, le=1.0)
    weight_interference: float = Field(default=0.15, ge=0.0, le=1.0)
    penalty_deadline: float = Field(default=3.0, ge=0.0)
    penalty_budget: float = Field(default=2.0, ge=0.0)
    workflow_yaml: Optional[str] = None


class Core(BaseModel):
    id: str
    index: int
    avail: float = 0.0


class Resource(BaseModel):
    id: str
    name: str
    kind: ResourceKind
    cores: List[Core]
    cpu: float
    memory: float
    price_per_cpu_second: float
    price_per_gb_second: float
    financial_network_price: float
    location: str
    status: ResourceStatus
    image_cache: List[str]


class Task(BaseModel):
    id: str
    label: str
    workflow_stage: str
    cpu: float
    memory: float
    base_runtime: float
    image: str
    runtime: Optional[str] = None
    run: Optional[str] = None
    predecessors: List[str]
    successors: List[str] = Field(default_factory=list)


class Dependency(BaseModel):
    source: str
    target: str
    data_mb: float


class SLA(BaseModel):
    deadline: float
    budget: float
    weight_time: float
    weight_cost: float
    weight_interference: float
    penalty_deadline: float
    penalty_budget: float


class Workflow(BaseModel):
    preset: str
    tasks: List[Task]
    dependencies: List[Dependency]
    predecessor_sets: Dict[str, List[str]]


class Matrices(BaseModel):
    et_0: Dict[str, Dict[str, float]]
    et_star: Dict[str, Dict[str, float]]
    interference_i_n: Dict[str, Dict[str, Dict[str, Dict[str, float]]]]
    bandwidth_bw: Dict[str, Dict[str, float]]
    transfer_delay: Dict[str, Dict[str, float]]
    financial_network_cost: Dict[str, Dict[str, float]]
    container_overhead: Dict[str, Dict[str, float]]


class ScoreBreakdown(BaseModel):
    time_score: float
    cost_score: float
    interference_score: float
    total_score: float


class PairwiseInterference(BaseModel):
    other_task_id: str
    value: float
    dimensions: Dict[str, float]


class CandidateEvaluation(BaseModel):
    task_id: str
    resource_id: str
    core_id: str
    rank: int
    selected: bool
    start_time: float
    finish_time: float
    base_runtime: float
    effective_runtime: float
    interference_time: float
    transfer_delay: float
    boot_overhead: float
    container_overhead: float
    predecessor_finish_floor: float
    raw_cost: float
    phi_n: float
    pairwise_interference: List[PairwiseInterference]
    score: ScoreBreakdown


class ScheduleStep(BaseModel):
    step: int
    task_id: str
    selected_resource_id: str
    selected_core_id: str
    selected_total_score: float
    candidates: List[CandidateEvaluation]


class Assignment(BaseModel):
    task_id: str
    resource_id: str
    core_id: str
    start_time: float
    finish_time: float
    effective_runtime: float
    transfer_delay: float
    boot_overhead: float
    container_overhead: float
    phi_n: float
    score: ScoreBreakdown
    predecessor_finish_floor: float


class CostVariables(BaseModel):
    c_cpu: Dict[str, float]
    c_mem: Dict[str, float]
    c_fin: Dict[str, float]
    fc: Dict[str, float]
    c_t_n: Dict[str, Dict[str, float]]
    b_used: float
    p_cc: float
    p_dl_w: float
    p_bud_w: float
    c_w: float


class TimingVariables(BaseModel):
    st: Dict[str, float]
    ft: Dict[str, float]
    avail: Dict[str, float]
    makespan: float
    transfer_delay_by_task: Dict[str, float]
    boot_overhead_by_task: Dict[str, float]
    container_overhead_by_task: Dict[str, float]


class InterferenceVariables(BaseModel):
    phi_n: Dict[str, float]
    et_star_by_task: Dict[str, float]
    colocated_tasks: Dict[str, List[str]]


class DeviationVariables(BaseModel):
    et_obs: Dict[str, float]
    var: Dict[str, float]
    excess: Dict[str, float]
    d_time: Dict[str, float]
    d_excess: Dict[str, float]
    d_n: Dict[str, float]
    d_w_time: float


class SchedulerVariables(BaseModel):
    x_t_n: Dict[str, str]
    f_t: Dict[str, str]
    s_n_t: Dict[str, float]
    avail_n: Dict[str, float]
    st: Dict[str, float]
    ft: Dict[str, float]
    makespan: float
    b_used: float


class SimulationResult(BaseModel):
    id: str
    seed: int
    workflow: Workflow
    resources: List[Resource]
    sla: SLA
    matrices: Matrices
    assignments: List[Assignment]
    scheduler_steps: List[ScheduleStep]
    scheduler_variables: SchedulerVariables
    timing_variables: TimingVariables
    cost_variables: CostVariables
    interference_variables: InterferenceVariables
    deviation_variables: DeviationVariables


class GeneratedSimulation(BaseModel):
    id: str
    seed: int
    workflow: Workflow
    resources: List[Resource]
    sla: SLA
    matrices: Matrices
