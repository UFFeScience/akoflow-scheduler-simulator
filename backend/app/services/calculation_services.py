from __future__ import annotations

from typing import Dict, List, Tuple

from app.models import Assignment, CostVariables, DeviationVariables, GeneratedSimulation, InterferenceVariables, PairwiseInterference, TimingVariables


class CalculateInterferenceService:
    def candidate_phi(self, generated: GeneratedSimulation, task_id: str, resource_id: str, scheduled: List[Assignment], start: float, finish: float) -> Tuple[float, List[str]]:
        phi, pairs = self.candidate_pairwise_interference(generated, task_id, resource_id, scheduled, start, finish)
        return phi, [pair.other_task_id for pair in pairs]

    def candidate_pairwise_interference(self, generated: GeneratedSimulation, task_id: str, resource_id: str, scheduled: List[Assignment], start: float, finish: float) -> Tuple[float, List[PairwiseInterference]]:
        colocated = [
            item.task_id
            for item in scheduled
            if item.resource_id == resource_id and max(start, item.start_time) < min(finish, item.finish_time)
        ]
        if not colocated:
            return 0.0, []
        total = 0.0
        count = 0
        pairs: List[PairwiseInterference] = []
        for other_id in colocated:
            dimensions: Dict[str, float] = {}
            for dimension in generated.matrices.interference_i_n[resource_id].values():
                value = dimension.get(other_id, {}).get(task_id, 0.0)
                total += value
                count += 1
            for dimension_name, dimension in generated.matrices.interference_i_n[resource_id].items():
                dimensions[dimension_name] = dimension.get(other_id, {}).get(task_id, 0.0)
            pair_value = round(sum(dimensions.values()) / max(len(dimensions), 1), 4)
            pairs.append(PairwiseInterference(other_task_id=other_id, value=pair_value, dimensions=dimensions))
        return round(total / max(count, 1), 4), pairs

    def execute(self, generated: GeneratedSimulation, assignments: List[Assignment]) -> InterferenceVariables:
        phi = {assignment.task_id: assignment.phi_n for assignment in assignments}
        et_star = {assignment.task_id: assignment.effective_runtime for assignment in assignments}
        colocated: Dict[str, List[str]] = {}
        for assignment in assignments:
            colocated[assignment.task_id] = [
                other.task_id
                for other in assignments
                if other.task_id != assignment.task_id
                and other.resource_id == assignment.resource_id
                and max(assignment.start_time, other.start_time) < min(assignment.finish_time, other.finish_time)
            ]
        return InterferenceVariables(phi_n=phi, et_star_by_task=et_star, colocated_tasks=colocated)


class CalculateTimingService:
    def execute(self, assignments: List[Assignment]) -> TimingVariables:
        st = {item.task_id: item.start_time for item in assignments}
        ft = {item.task_id: item.finish_time for item in assignments}
        avail = {}
        for item in assignments:
            avail[item.core_id] = max(avail.get(item.core_id, 0.0), item.finish_time)
        return TimingVariables(
            st=st,
            ft=ft,
            avail=avail,
            makespan=round(max(ft.values()) if ft else 0.0, 3),
            transfer_delay_by_task={item.task_id: item.transfer_delay for item in assignments},
            boot_overhead_by_task={item.task_id: item.boot_overhead for item in assignments},
            container_overhead_by_task={item.task_id: item.container_overhead for item in assignments},
        )


class CalculateCostService:
    def execute(self, generated: GeneratedSimulation, assignments: List[Assignment], makespan: float) -> CostVariables:
        resources = {resource.id: resource for resource in generated.resources}
        tasks = {task.id: task for task in generated.workflow.tasks}
        assignment_by_task = {item.task_id: item for item in assignments}
        c_cpu: Dict[str, float] = {}
        c_mem: Dict[str, float] = {}
        c_fin: Dict[str, float] = {}
        fc: Dict[str, float] = {}
        c_t_n: Dict[str, Dict[str, float]] = {}

        for task in generated.workflow.tasks:
            c_t_n[task.id] = {}
            for resource in generated.resources:
                runtime = generated.matrices.et_star[task.id][resource.id] + generated.matrices.container_overhead[task.id][resource.id]
                c_t_n[task.id][resource.id] = round(
                    runtime * (task.cpu * resource.price_per_cpu_second + task.memory * resource.price_per_gb_second),
                    4,
                )

        for assignment in assignments:
            task = tasks[assignment.task_id]
            resource = resources[assignment.resource_id]
            c_cpu[task.id] = round(assignment.effective_runtime * task.cpu * resource.price_per_cpu_second, 4)
            c_mem[task.id] = round(assignment.effective_runtime * task.memory * resource.price_per_gb_second, 4)
            network_cost = 0.0
            for dependency in generated.workflow.dependencies:
                if dependency.target != task.id:
                    continue
                predecessor = assignment_by_task[dependency.source]
                unit_price = generated.matrices.financial_network_cost[predecessor.resource_id][assignment.resource_id]
                network_cost += dependency.data_mb * unit_price
            c_fin[task.id] = round(network_cost, 4)
            fc[task.id] = round(c_cpu[task.id] + c_mem[task.id] + c_fin[task.id], 4)

        b_used = round(sum(fc.values()), 4)
        p_cc = round(sum(c_fin.values()), 4)
        return CostVariables(
            c_cpu=c_cpu,
            c_mem=c_mem,
            c_fin=c_fin,
            fc=fc,
            c_t_n=c_t_n,
            b_used=b_used,
            p_cc=p_cc,
            c_w=round(b_used + p_cc, 4),
        )


class CalculateDeviationService:
    def execute(self, generated: GeneratedSimulation, assignments: List[Assignment]) -> DeviationVariables:
        et_obs: Dict[str, float] = {}
        var: Dict[str, float] = {}
        excess: Dict[str, float] = {}
        d_time: Dict[str, float] = {}
        d_excess: Dict[str, float] = {}
        d_n: Dict[str, float] = {}
        resource_excess_values: Dict[str, List[float]] = {}

        for index, assignment in enumerate(assignments):
            drift = ((generated.seed + index * 17) % 13 - 6) / 100.0
            observed = round(assignment.effective_runtime * (1.0 + drift + assignment.phi_n * 0.2), 3)
            base_runtime = generated.matrices.et_0[assignment.task_id][assignment.resource_id]
            variance = round(observed - base_runtime, 3)
            over = round(max(0.0, variance), 3)
            et_obs[assignment.task_id] = observed
            var[assignment.task_id] = variance
            excess[assignment.task_id] = over
            d_time[assignment.task_id] = round(variance / max(base_runtime, 0.001), 4)
            d_excess[assignment.task_id] = round(over / max(base_runtime, 0.001), 4)
            resource_excess_values.setdefault(assignment.resource_id, []).append(d_excess[assignment.task_id])

        for resource_id, values in resource_excess_values.items():
            d_n[resource_id] = round(sum(values) / max(len(values), 1), 4)

        return DeviationVariables(
            et_obs=et_obs,
            var=var,
            excess=excess,
            d_time=d_time,
            d_excess=d_excess,
            d_n=d_n,
            d_w_time=round(sum(d_excess.values()) / max(len(d_excess), 1), 4),
        )
