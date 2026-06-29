from __future__ import annotations

from typing import Dict, List, Set

from app.models import Assignment, CandidateEvaluation, GeneratedSimulation, ScheduleStep, SchedulerVariables, ScoreBreakdown, SimulationResult
from app.services.calculation_services import (
    CalculateCostService,
    CalculateDeviationService,
    CalculateInterferenceService,
    CalculateTimingService,
)


class ScheduleWorkflowService:
    def __init__(self) -> None:
        self.interference_service = CalculateInterferenceService()
        self.timing_service = CalculateTimingService()
        self.cost_service = CalculateCostService()
        self.deviation_service = CalculateDeviationService()

    def execute(self, generated: GeneratedSimulation) -> SimulationResult:
        tasks = {task.id: task for task in generated.workflow.tasks}
        resources = {resource.id: resource for resource in generated.resources}
        dependencies_by_target = {}
        for dependency in generated.workflow.dependencies:
            dependencies_by_target.setdefault(dependency.target, []).append(dependency)

        core_avail = {core.id: 0.0 for resource in generated.resources for core in resource.cores}
        node_has_booted = {resource.id: resource.status == "warm" for resource in generated.resources}
        assignments: List[Assignment] = []
        assignment_by_task: Dict[str, Assignment] = {}
        scheduler_steps: List[ScheduleStep] = []

        for step_index, task_id in enumerate(self._topological_order(generated), start=1):
            task = tasks[task_id]
            candidates = []
            candidate_evaluations: List[CandidateEvaluation] = []
            for resource in generated.resources:
                if generated.sla.budget == 0 and resource.kind == "cloud":
                    continue
                if task.cpu > resource.cpu or task.memory > resource.memory:
                    continue
                predecessor_floor = 0.0
                transfer_total = 0.0
                for dependency in dependencies_by_target.get(task.id, []):
                    predecessor = assignment_by_task[dependency.source]
                    transfer = 0.0
                    if predecessor.resource_id != resource.id:
                        bandwidth = generated.matrices.bandwidth_bw[predecessor.resource_id][resource.id]
                        transfer = dependency.data_mb / bandwidth
                    transfer_total += transfer
                    predecessor_floor = max(predecessor_floor, predecessor.finish_time + transfer)

                for core in resource.cores:
                    boot = 0.0 if node_has_booted[resource.id] else (8.0 if resource.kind == "cloud" else 3.0)
                    container = generated.matrices.container_overhead[task.id][resource.id]
                    tentative_start = round(max(predecessor_floor, core_avail[core.id]) + boot + container, 3)
                    base_runtime = generated.matrices.et_0[task.id][resource.id]
                    phi, pairwise_interference = self.interference_service.candidate_pairwise_interference(
                        generated,
                        task.id,
                        resource.id,
                        assignments,
                        tentative_start,
                        tentative_start + base_runtime,
                    )
                    effective_runtime = round(base_runtime * (1.0 + phi), 3)
                    interference_time = round(effective_runtime - base_runtime, 3)
                    finish = round(tentative_start + effective_runtime, 3)
                    raw_cost = effective_runtime * (
                        task.cpu * resource.price_per_cpu_second + task.memory * resource.price_per_gb_second
                    )
                    time_score = finish / max(generated.sla.deadline, 0.001)
                    cost_score = 0.0 if generated.sla.budget == 0 else raw_cost / generated.sla.budget
                    interference_score = phi
                    total_score = (
                        generated.sla.weight_time * time_score
                        + generated.sla.weight_cost * cost_score
                        + generated.sla.weight_interference * interference_score
                    )
                    score = ScoreBreakdown(
                        time_score=round(time_score, 5),
                        cost_score=round(cost_score, 5),
                        interference_score=round(interference_score, 5),
                        total_score=round(total_score, 5),
                    )
                    assignment = Assignment(
                        task_id=task.id,
                        resource_id=resource.id,
                        core_id=core.id,
                        start_time=tentative_start,
                        finish_time=finish,
                        effective_runtime=effective_runtime,
                        transfer_delay=round(transfer_total, 3),
                        boot_overhead=boot,
                        container_overhead=container,
                        phi_n=phi,
                        predecessor_finish_floor=round(predecessor_floor, 3),
                        score=score,
                    )
                    candidates.append(assignment)
                    candidate_evaluations.append(
                        CandidateEvaluation(
                            task_id=task.id,
                            resource_id=resource.id,
                            core_id=core.id,
                            rank=0,
                            selected=False,
                            start_time=tentative_start,
                            finish_time=finish,
                            base_runtime=base_runtime,
                            effective_runtime=effective_runtime,
                            interference_time=interference_time,
                            transfer_delay=round(transfer_total, 3),
                            boot_overhead=boot,
                            container_overhead=container,
                            predecessor_finish_floor=round(predecessor_floor, 3),
                            raw_cost=round(raw_cost, 4),
                            phi_n=phi,
                            pairwise_interference=pairwise_interference,
                            score=score,
                        )
                    )
            if not candidates:
                raise ValueError(f"No feasible resource for task {task.id}")
            selected = min(candidates, key=lambda item: (item.score.total_score, item.finish_time, item.resource_id, item.core_id))
            ranked_candidates = sorted(
                candidate_evaluations,
                key=lambda item: (item.score.total_score, item.finish_time, item.resource_id, item.core_id),
            )
            for rank, candidate in enumerate(ranked_candidates, start=1):
                candidate.rank = rank
                candidate.selected = candidate.resource_id == selected.resource_id and candidate.core_id == selected.core_id
            scheduler_steps.append(
                ScheduleStep(
                    step=step_index,
                    task_id=task.id,
                    selected_resource_id=selected.resource_id,
                    selected_core_id=selected.core_id,
                    selected_total_score=selected.score.total_score,
                    candidates=ranked_candidates,
                )
            )
            assignments.append(selected)
            assignment_by_task[selected.task_id] = selected
            core_avail[selected.core_id] = selected.finish_time
            node_has_booted[selected.resource_id] = True
            generated.matrices.et_star[selected.task_id][selected.resource_id] = selected.effective_runtime

        timing = self.timing_service.execute(assignments)
        costs = self.cost_service.execute(generated, assignments, timing.makespan)
        interference = self.interference_service.execute(generated, assignments)
        deviation = self.deviation_service.execute(generated, assignments)
        scheduler = SchedulerVariables(
            x_t_n={item.task_id: item.resource_id for item in assignments},
            f_t={item.task_id: item.core_id for item in assignments},
            s_n_t={item.task_id: item.start_time for item in assignments},
            avail_n={resource_id: max((item.finish_time for item in assignments if item.resource_id == resource_id), default=0.0) for resource_id in resources},
            st=timing.st,
            ft=timing.ft,
            makespan=timing.makespan,
            b_used=costs.b_used,
        )
        return SimulationResult(
            id=generated.id,
            seed=generated.seed,
            workflow=generated.workflow,
            resources=generated.resources,
            sla=generated.sla,
            matrices=generated.matrices,
            assignments=assignments,
            scheduler_steps=scheduler_steps,
            scheduler_variables=scheduler,
            timing_variables=timing,
            cost_variables=costs,
            interference_variables=interference,
            deviation_variables=deviation,
        )

    def _topological_order(self, generated: GeneratedSimulation) -> List[str]:
        remaining: Set[str] = {task.id for task in generated.workflow.tasks}
        predecessors = {task.id: set(task.predecessors) for task in generated.workflow.tasks}
        order: List[str] = []
        while remaining:
            ready = sorted(task_id for task_id in remaining if predecessors[task_id].issubset(order))
            if not ready:
                raise ValueError("Workflow contains a cycle")
            for task_id in ready:
                order.append(task_id)
                remaining.remove(task_id)
        return order
