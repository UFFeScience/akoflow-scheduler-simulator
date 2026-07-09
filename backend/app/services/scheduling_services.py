from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, List, Optional, Set

from app.models import Assignment, CandidateEvaluation, GeneratedSimulation, MachineStopInterval, ScheduleConstraints, ScheduleOptimizationResponse, ScheduleOption, ScheduleStep, SchedulerVariables, ScoreBreakdown, SimulationResult
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
        node_ready_time = {resource.id: 0.0 for resource in generated.resources}
        node_last_active_time = {resource.id: 0.0 for resource in generated.resources}
        node_stop_intervals: List[MachineStopInterval] = []
        assignments: List[Assignment] = []
        assignment_by_task: Dict[str, Assignment] = {}
        scheduler_steps: List[ScheduleStep] = []

        for step_index, task_id in enumerate(self._topological_order(generated), start=1):
            task = tasks[task_id]
            candidates = []
            candidate_evaluations: List[CandidateEvaluation] = []
            candidate_scores: List[tuple[Assignment, CandidateEvaluation, float, float, float]] = []
            for resource in generated.resources:
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
                    ready_floor = max(predecessor_floor, core_avail[core.id], node_ready_time[resource.id])
                    last_active_time = node_last_active_time[resource.id]
                    stop_boot = (
                        resource.kind == "cloud"
                        and node_has_booted[resource.id]
                        and resource.boot_overhead > 0
                        and ready_floor - last_active_time >= resource.boot_overhead
                    )
                    cold_boot = not node_has_booted[resource.id]
                    boot = resource.boot_overhead if cold_boot or stop_boot else 0.0
                    container = generated.matrices.container_overhead[task.id][resource.id]
                    boot_ready_time = ready_floor + (boot if cold_boot else 0.0)
                    tentative_start = round(boot_ready_time + container, 3)
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
                    score = ScoreBreakdown(
                        time_score=0.0,
                        cost_score=0.0,
                        interference_score=0.0,
                        total_score=0.0,
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
                    candidate = CandidateEvaluation(
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
                    candidates.append(assignment)
                    candidate_evaluations.append(candidate)
                    candidate_scores.append((assignment, candidate, finish, raw_cost, phi))
            if not candidates:
                raise ValueError(f"No feasible resource for task {task.id}")
            max_finish = max(finish for _, _, finish, _, _ in candidate_scores)
            max_raw_cost = max(raw_cost for _, _, _, raw_cost, _ in candidate_scores)
            for assignment, candidate, finish, raw_cost, phi in candidate_scores:
                time_score = finish / max(max_finish, 0.001)
                cost_score = 0.0 if max_raw_cost == 0 else raw_cost / max_raw_cost
                interference_score = phi
                total_score = (
                    generated.sla.weight_time * time_score
                    + generated.sla.weight_cost * cost_score
                )
                score = ScoreBreakdown(
                    time_score=round(time_score, 5),
                    cost_score=round(cost_score, 5),
                    interference_score=round(interference_score, 5),
                    total_score=round(total_score, 5),
                )
                assignment.score = score
                candidate.score = score
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
            if selected.boot_overhead > 0:
                resource = resources[selected.resource_id]
                previous_active_time = node_last_active_time[selected.resource_id]
                cold_boot = not node_has_booted[selected.resource_id]
                boot_finish_time = round(selected.start_time - selected.container_overhead, 3)
                if (
                    resource.kind == "cloud"
                    and node_has_booted[selected.resource_id]
                    and boot_finish_time - previous_active_time >= selected.boot_overhead
                ):
                    node_stop_intervals.append(
                        MachineStopInterval(
                            resource_id=selected.resource_id,
                            stop_time=round(previous_active_time, 3),
                            boot_start_time=round(boot_finish_time - selected.boot_overhead, 3),
                            boot_finish_time=boot_finish_time,
                            boot_overhead=selected.boot_overhead,
                            reason=f"idle gap paid boot before {selected.task_id}",
                        )
                    )
                node_ready_time[selected.resource_id] = max(
                    node_ready_time[selected.resource_id],
                    boot_finish_time,
                )
            node_has_booted[selected.resource_id] = True
            node_last_active_time[selected.resource_id] = max(node_last_active_time[selected.resource_id], selected.finish_time)
            generated.matrices.et_star[selected.task_id][selected.resource_id] = selected.effective_runtime

        return self.build_result(generated, assignments, node_stop_intervals, scheduler_steps)

    def build_result(
        self,
        generated: GeneratedSimulation,
        assignments: List[Assignment],
        node_stop_intervals: List[MachineStopInterval],
        scheduler_steps: List[ScheduleStep],
    ) -> SimulationResult:
        resources = {resource.id: resource for resource in generated.resources}
        for assignment in assignments:
            generated.matrices.et_star[assignment.task_id][assignment.resource_id] = assignment.effective_runtime

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
            machine_stop_intervals=node_stop_intervals,
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


@dataclass
class BeamState:
    assignments: List[Assignment]
    assignment_by_task: Dict[str, Assignment]
    core_avail: Dict[str, float]
    node_has_booted: Dict[str, bool]
    node_ready_time: Dict[str, float]
    node_last_active_time: Dict[str, float]
    node_stop_intervals: List[MachineStopInterval]
    scheduler_steps: List[ScheduleStep]
    partial_budget_used: float
    partial_makespan: float
    partial_score: float


class BeamScheduleOptimizerService:
    def __init__(self) -> None:
        self.scheduler = ScheduleWorkflowService()
        self.interference_service = self.scheduler.interference_service

    def execute(self, generated: GeneratedSimulation) -> ScheduleOptimizationResponse:
        option_count = max(1, min(generated.sla.option_count, 100))
        budget_limit = generated.sla.budget_limit
        deadline_limit = generated.sla.deadline_limit
        final_states = self._search(generated, option_count)
        options = self._build_options(generated, final_states, option_count, budget_limit, deadline_limit)
        return ScheduleOptimizationResponse(
            selected_option_id=options[0].id if options else None,
            constraints=ScheduleConstraints(
                budget_limit=budget_limit,
                deadline_limit=deadline_limit,
                option_count=option_count,
            ),
            options=options,
        )

    def _search(self, generated: GeneratedSimulation, option_count: int) -> List[BeamState]:
        core_avail = {core.id: 0.0 for resource in generated.resources for core in resource.cores}
        initial = BeamState(
            assignments=[],
            assignment_by_task={},
            core_avail=core_avail,
            node_has_booted={resource.id: resource.status == "warm" for resource in generated.resources},
            node_ready_time={resource.id: 0.0 for resource in generated.resources},
            node_last_active_time={resource.id: 0.0 for resource in generated.resources},
            node_stop_intervals=[],
            scheduler_steps=[],
            partial_budget_used=0.0,
            partial_makespan=0.0,
            partial_score=0.0,
        )
        beam: List[BeamState] = [initial]
        overflow: List[BeamState] = []
        beam_width = min(600, max(120, option_count * 6))
        overflow_width = min(240, max(60, option_count * 3))

        for step_index, task_id in enumerate(self.scheduler._topological_order(generated), start=1):
            expanded: List[BeamState] = []
            overflow_expanded: List[BeamState] = []
            for state in beam + overflow:
                next_states = self._expand_state(generated, state, step_index, task_id)
                for next_state in next_states:
                    if generated.sla.budget_limit is not None and next_state.partial_budget_used > generated.sla.budget_limit:
                        overflow_expanded.append(next_state)
                    else:
                        expanded.append(next_state)
            if not expanded and not overflow_expanded:
                raise ValueError(f"No feasible resource for task {task_id}")
            beam = self._select_beam_states(expanded, beam_width, generated)
            overflow = self._select_beam_states(overflow_expanded, overflow_width, generated)
            if not beam:
                overflow = self._select_beam_states(overflow, overflow_width, generated)

        return beam + overflow

    def _select_beam_states(self, states: List[BeamState], width: int, generated: GeneratedSimulation) -> List[BeamState]:
        if width <= 0 or not states:
            return []
        unique = self._dedupe_states_by_signature(states)
        if len(unique) <= width:
            return unique

        selected: List[BeamState] = []
        selected_signatures: Set[tuple[tuple[str, str], ...]] = set()

        strategy_lists = [
            sorted(unique, key=lambda state: (state.partial_makespan, state.partial_budget_used, state.partial_score)),
            sorted(unique, key=lambda state: (state.partial_budget_used, state.partial_makespan, state.partial_score)),
            sorted(unique, key=self._partial_rank_key),
            sorted(unique, key=self._machine_diversity_rank_key),
            sorted(unique, key=lambda state: self._limit_proximity_key(state, generated)),
        ]

        cursor = [0 for _ in strategy_lists]
        while len(selected) < width and any(index < len(strategy_lists[pos]) for pos, index in enumerate(cursor)):
            progressed = False
            for pos, candidates in enumerate(strategy_lists):
                while cursor[pos] < len(candidates):
                    state = candidates[cursor[pos]]
                    cursor[pos] += 1
                    signature = self._state_signature(state)
                    if signature in selected_signatures:
                        continue
                    selected.append(state)
                    selected_signatures.add(signature)
                    progressed = True
                    break
                if len(selected) >= width:
                    break
            if not progressed:
                break

        while len(selected) < width:
            candidate = self._farthest_state(unique, selected, selected_signatures)
            if candidate is None:
                break
            selected.append(candidate)
            selected_signatures.add(self._state_signature(candidate))

        return selected

    def _expand_state(self, generated: GeneratedSimulation, state: BeamState, step_index: int, task_id: str) -> List[BeamState]:
        tasks = {task.id: task for task in generated.workflow.tasks}
        resources = {resource.id: resource for resource in generated.resources}
        dependencies_by_target = {}
        for dependency in generated.workflow.dependencies:
            dependencies_by_target.setdefault(dependency.target, []).append(dependency)

        task = tasks[task_id]
        candidate_rows: List[tuple[Assignment, CandidateEvaluation, float, float, float, float]] = []
        for resource in generated.resources:
            if task.cpu > resource.cpu or task.memory > resource.memory:
                continue
            predecessor_floor = 0.0
            transfer_total = 0.0
            network_cost = 0.0
            for dependency in dependencies_by_target.get(task.id, []):
                predecessor = state.assignment_by_task[dependency.source]
                transfer = 0.0
                if predecessor.resource_id != resource.id:
                    bandwidth = generated.matrices.bandwidth_bw[predecessor.resource_id][resource.id]
                    transfer = dependency.data_mb / bandwidth
                transfer_total += transfer
                predecessor_floor = max(predecessor_floor, predecessor.finish_time + transfer)
                unit_price = generated.matrices.financial_network_cost[predecessor.resource_id][resource.id]
                network_cost += dependency.data_mb * unit_price

            for core in resource.cores:
                ready_floor = max(predecessor_floor, state.core_avail[core.id], state.node_ready_time[resource.id])
                last_active_time = state.node_last_active_time[resource.id]
                stop_boot = (
                    resource.kind == "cloud"
                    and state.node_has_booted[resource.id]
                    and resource.boot_overhead > 0
                    and ready_floor - last_active_time >= resource.boot_overhead
                )
                cold_boot = not state.node_has_booted[resource.id]
                boot = resource.boot_overhead if cold_boot or stop_boot else 0.0
                container = generated.matrices.container_overhead[task.id][resource.id]
                boot_ready_time = ready_floor + (boot if cold_boot else 0.0)
                tentative_start = round(boot_ready_time + container, 3)
                base_runtime = generated.matrices.et_0[task.id][resource.id]
                phi, pairwise_interference = self.interference_service.candidate_pairwise_interference(
                    generated,
                    task.id,
                    resource.id,
                    state.assignments,
                    tentative_start,
                    tentative_start + base_runtime,
                )
                effective_runtime = round(base_runtime * (1.0 + phi), 3)
                finish = round(tentative_start + effective_runtime, 3)
                raw_cost = effective_runtime * (
                    task.cpu * resource.price_per_cpu_second + task.memory * resource.price_per_gb_second
                )
                incremental_budget = raw_cost + network_cost
                score = ScoreBreakdown(time_score=0.0, cost_score=0.0, interference_score=0.0, total_score=0.0)
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
                candidate = CandidateEvaluation(
                    task_id=task.id,
                    resource_id=resource.id,
                    core_id=core.id,
                    rank=0,
                    selected=False,
                    start_time=tentative_start,
                    finish_time=finish,
                    base_runtime=base_runtime,
                    effective_runtime=effective_runtime,
                    interference_time=round(effective_runtime - base_runtime, 3),
                    transfer_delay=round(transfer_total, 3),
                    boot_overhead=boot,
                    container_overhead=container,
                    predecessor_finish_floor=round(predecessor_floor, 3),
                    raw_cost=round(raw_cost, 4),
                    phi_n=phi,
                    pairwise_interference=pairwise_interference,
                    score=score,
                )
                candidate_rows.append((assignment, candidate, finish, raw_cost, phi, incremental_budget))
        if not candidate_rows:
            return []

        max_finish = max(finish for _, _, finish, _, _, _ in candidate_rows)
        max_raw_cost = max(raw_cost for _, _, _, raw_cost, _, _ in candidate_rows)
        scored_rows = []
        for assignment, candidate, finish, raw_cost, phi, incremental_budget in candidate_rows:
            time_score = finish / max(max_finish, 0.001)
            cost_score = 0.0 if max_raw_cost == 0 else raw_cost / max_raw_cost
            score = ScoreBreakdown(
                time_score=round(time_score, 5),
                cost_score=round(cost_score, 5),
                interference_score=round(phi, 5),
                total_score=round(generated.sla.weight_time * time_score + generated.sla.weight_cost * cost_score, 5),
            )
            assignment.score = score
            candidate.score = score
            scored_rows.append((assignment, candidate, incremental_budget))

        ranked_candidates = sorted(
            [candidate for _, candidate, _ in scored_rows],
            key=lambda item: (item.score.total_score, item.finish_time, item.resource_id, item.core_id),
        )
        rank_by_slot = {}
        for rank, candidate in enumerate(ranked_candidates, start=1):
            rank_by_slot[(candidate.resource_id, candidate.core_id)] = rank

        next_states: List[BeamState] = []
        for assignment, candidate, incremental_budget in scored_rows:
            selected_candidates = []
            for ranked in ranked_candidates:
                is_selected = ranked.resource_id == assignment.resource_id and ranked.core_id == assignment.core_id
                selected_candidates.append(ranked.model_copy(update={"rank": rank_by_slot[(ranked.resource_id, ranked.core_id)], "selected": is_selected}))
            selected_rank = rank_by_slot[(assignment.resource_id, assignment.core_id)]
            step = ScheduleStep(
                step=step_index,
                task_id=task.id,
                selected_resource_id=assignment.resource_id,
                selected_core_id=assignment.core_id,
                selected_total_score=assignment.score.total_score,
                candidates=selected_candidates,
            )

            core_avail = dict(state.core_avail)
            core_avail[assignment.core_id] = assignment.finish_time
            node_has_booted = dict(state.node_has_booted)
            node_ready_time = dict(state.node_ready_time)
            node_last_active_time = dict(state.node_last_active_time)
            node_stop_intervals = list(state.node_stop_intervals)
            if assignment.boot_overhead > 0:
                resource = resources[assignment.resource_id]
                previous_active_time = node_last_active_time[assignment.resource_id]
                boot_finish_time = round(assignment.start_time - assignment.container_overhead, 3)
                if (
                    resource.kind == "cloud"
                    and node_has_booted[assignment.resource_id]
                    and boot_finish_time - previous_active_time >= assignment.boot_overhead
                ):
                    node_stop_intervals.append(
                        MachineStopInterval(
                            resource_id=assignment.resource_id,
                            stop_time=round(previous_active_time, 3),
                            boot_start_time=round(boot_finish_time - assignment.boot_overhead, 3),
                            boot_finish_time=boot_finish_time,
                            boot_overhead=assignment.boot_overhead,
                            reason=f"idle gap paid boot before {assignment.task_id}",
                        )
                    )
                node_ready_time[assignment.resource_id] = max(node_ready_time[assignment.resource_id], boot_finish_time)
            node_has_booted[assignment.resource_id] = True
            node_last_active_time[assignment.resource_id] = max(node_last_active_time[assignment.resource_id], assignment.finish_time)
            partial_budget_used = round(state.partial_budget_used + incremental_budget, 4)
            partial_makespan = round(max(state.partial_makespan, assignment.finish_time), 3)
            deadline_pressure = self._limit_ratio(partial_makespan, generated.sla.deadline_limit)
            budget_pressure = self._limit_ratio(partial_budget_used, generated.sla.budget_limit)
            partial_score = round(state.partial_score + assignment.score.total_score + deadline_pressure + budget_pressure + selected_rank * 0.0001, 6)
            next_assignments = state.assignments + [assignment.model_copy(deep=True)]
            next_states.append(
                BeamState(
                    assignments=next_assignments,
                    assignment_by_task={**state.assignment_by_task, assignment.task_id: next_assignments[-1]},
                    core_avail=core_avail,
                    node_has_booted=node_has_booted,
                    node_ready_time=node_ready_time,
                    node_last_active_time=node_last_active_time,
                    node_stop_intervals=node_stop_intervals,
                    scheduler_steps=state.scheduler_steps + [step],
                    partial_budget_used=partial_budget_used,
                    partial_makespan=partial_makespan,
                    partial_score=partial_score,
                )
            )
        return next_states

    def _build_options(
        self,
        generated: GeneratedSimulation,
        states: List[BeamState],
        option_count: int,
        budget_limit: Optional[float],
        deadline_limit: Optional[float],
    ) -> List[ScheduleOption]:
        unique_states = self._dedupe_states_by_signature(states)
        built: List[ScheduleOption] = []
        for state in unique_states:
            result = self.scheduler.build_result(
                generated.model_copy(deep=True),
                [assignment.model_copy(deep=True) for assignment in state.assignments],
                [item.model_copy(deep=True) for item in state.node_stop_intervals],
                [step.model_copy(deep=True) for step in state.scheduler_steps],
            )
            budget_used = result.cost_variables.b_used
            makespan = result.timing_variables.makespan
            budget_violation = 0.0 if budget_limit is None else round(max(0.0, budget_used - budget_limit), 4)
            deadline_violation = 0.0 if deadline_limit is None else round(max(0.0, makespan - deadline_limit), 3)
            feasible = budget_violation == 0.0 and deadline_violation == 0.0
            machine_distribution = self._machine_distribution(state)
            built.append(
                ScheduleOption(
                    id="pending",
                    rank=0,
                    feasible=feasible,
                    recommended=False,
                    budget_used=budget_used,
                    budget_limit=budget_limit,
                    budget_violation=budget_violation,
                    makespan=makespan,
                    deadline_limit=deadline_limit,
                    deadline_violation=deadline_violation,
                    machine_signature=self._machine_signature_text(state),
                    machine_distribution=machine_distribution,
                    weighted_score=0.0,
                    diversity_score=0.0,
                    result=result,
                )
            )

        feasible_options = [option for option in built if option.feasible]
        source = feasible_options if feasible_options else built
        self._annotate_option_scores(source, generated)
        ranked = self._rank_options(source, option_count)
        selected = ranked[:option_count]
        for rank, option in enumerate(selected, start=1):
            option.rank = rank
            option.id = f"option-{rank}"
            option.recommended = rank == 1
        return selected

    def _rank_options(self, options: List[ScheduleOption], option_count: int) -> List[ScheduleOption]:
        if not options:
            return []
        preferred = min(options, key=lambda option: (option.weighted_score, option.makespan, option.budget_used))
        selected = [preferred]
        remaining = [option for option in options if option is not preferred]

        while remaining and len(selected) < option_count:
            candidate = max(
                remaining,
                key=lambda option: (
                    self._option_distance_to_selected(option, selected),
                    -option.weighted_score,
                    option.diversity_score,
                    -option.makespan,
                    -option.budget_used,
                ),
            )
            selected.append(candidate)
            remaining.remove(candidate)

        if len(selected) < option_count:
            selected.extend(sorted(remaining, key=lambda option: (option.weighted_score, option.makespan, option.budget_used))[: option_count - len(selected)])
        return selected

    def _annotate_option_scores(self, options: List[ScheduleOption], generated: GeneratedSimulation) -> None:
        max_budget = max((option.budget_used for option in options), default=0.0) or 1.0
        max_makespan = max((option.makespan for option in options), default=0.0) or 1.0
        for option in options:
            time_score = option.makespan / max_makespan
            cost_score = option.budget_used / max_budget
            violation_penalty = option.budget_violation + option.deadline_violation
            option.weighted_score = round(
                generated.sla.weight_time * time_score
                + generated.sla.weight_cost * cost_score
                + violation_penalty,
                6,
            )
            option.diversity_score = round(self._distribution_diversity(option.machine_distribution), 6)

    def _partial_rank_key(self, state: BeamState) -> tuple[float, float, float]:
        return (state.partial_score, state.partial_makespan, state.partial_budget_used)

    def _limit_ratio(self, value: float, limit: Optional[float]) -> float:
        if limit is None or limit <= 0:
            return 0.0
        return max(0.0, value / limit - 1.0)

    def _limit_proximity_key(self, state: BeamState, generated: GeneratedSimulation) -> tuple[float, float, float]:
        budget_target = self._target_ratio_distance(state.partial_budget_used, generated.sla.budget_limit)
        deadline_target = self._target_ratio_distance(state.partial_makespan, generated.sla.deadline_limit)
        return (budget_target + deadline_target, state.partial_score, state.partial_makespan)

    def _target_ratio_distance(self, value: float, limit: Optional[float]) -> float:
        if limit is None or limit <= 0:
            return 0.0
        return abs((value / limit) - 0.8)

    def _machine_diversity_rank_key(self, state: BeamState) -> tuple[int, int, float]:
        distribution = self._machine_distribution(state)
        return (-len(distribution), max(distribution.values(), default=0), state.partial_score)

    def _dedupe_states_by_signature(self, states: List[BeamState]) -> List[BeamState]:
        unique: List[BeamState] = []
        seen: Set[tuple[tuple[str, str], ...]] = set()
        for state in sorted(states, key=self._partial_rank_key):
            signature = self._state_signature(state)
            if signature in seen:
                continue
            seen.add(signature)
            unique.append(state)
        return unique

    def _state_signature(self, state: BeamState) -> tuple[tuple[str, str], ...]:
        return tuple((assignment.task_id, assignment.resource_id) for assignment in state.assignments)

    def _machine_signature_text(self, state: BeamState) -> str:
        return "|".join(f"{assignment.task_id}:{assignment.resource_id}" for assignment in state.assignments)

    def _machine_distribution(self, state: BeamState) -> Dict[str, int]:
        distribution: Dict[str, int] = {}
        for assignment in state.assignments:
            distribution[assignment.resource_id] = distribution.get(assignment.resource_id, 0) + 1
        return distribution

    def _distribution_diversity(self, distribution: Dict[str, int]) -> float:
        if not distribution:
            return 0.0
        total = sum(distribution.values())
        max_share = max(distribution.values()) / max(total, 1)
        return len(distribution) - max_share

    def _farthest_state(
        self,
        candidates: List[BeamState],
        selected: List[BeamState],
        selected_signatures: Set[tuple[tuple[str, str], ...]],
    ) -> Optional[BeamState]:
        best: Optional[BeamState] = None
        best_key: Optional[tuple[float, float, float]] = None
        for candidate in candidates:
            signature = self._state_signature(candidate)
            if signature in selected_signatures:
                continue
            distance = self._state_distance_to_selected(candidate, selected)
            key = (distance, -candidate.partial_score, -candidate.partial_makespan)
            if best_key is None or key > best_key:
                best = candidate
                best_key = key
        return best

    def _state_distance_to_selected(self, candidate: BeamState, selected: List[BeamState]) -> float:
        if not selected:
            return float(len(candidate.assignments))
        return min(self._state_distance(candidate, item) for item in selected)

    def _state_distance(self, left: BeamState, right: BeamState) -> float:
        left_slots = {(assignment.task_id, assignment.resource_id) for assignment in left.assignments}
        right_slots = {(assignment.task_id, assignment.resource_id) for assignment in right.assignments}
        return float(len(left_slots.symmetric_difference(right_slots)))

    def _option_distance_to_selected(self, candidate: ScheduleOption, selected: List[ScheduleOption]) -> float:
        candidate_slots = set(candidate.machine_signature.split("|")) if candidate.machine_signature else set()
        distances = []
        for option in selected:
            option_slots = set(option.machine_signature.split("|")) if option.machine_signature else set()
            distances.append(float(len(candidate_slots.symmetric_difference(option_slots))))
        return min(distances) if distances else float(len(candidate_slots))
