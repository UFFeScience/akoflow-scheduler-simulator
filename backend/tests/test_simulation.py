from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.models import SimulationRequest
from app.services.generation_services import GenerateSimulationService
from app.services.scheduling_services import BeamScheduleOptimizerService, ScheduleWorkflowService


AKOFLOW_YAML = """
name: wf-synthetic-dag
spec:
  runtime: hpcplafrim
  image: /home/wferreir/wf-synthetic-tar/sifs/akoflow-wf-montage_050d.sif
  activities:
    - name: mprojectid0000001
      runtime: hpcplafrim
      run: mv -fvu /akoflow-wfa-shared/* . && mProject -X a.fits pa.fits region.hdr
      memoryLimit: 256M
      cpuLimit: 1
    - name: mprojectid0000002
      runtime: hpcplafrim
      run: mv -fvu /akoflow-wfa-shared/* . && mProject -X b.fits pb.fits region.hdr
      memoryLimit: 256M
      cpuLimit: 1
    - name: mdifffitid0000005
      runtime: hpcplafrim
      run: mDiffFit -d -s fit.txt pa.fits pb.fits diff.fits region.hdr
      memoryLimit: 256M
      cpuLimit: 1
      dependsOn:
        - mprojectid0000001
        - mprojectid0000002
"""


def run_default(seed: int = 42):
    request = SimulationRequest(seed=seed, task_count=14, edge_density=0.3)
    generated = GenerateSimulationService().execute(request)
    result = ScheduleWorkflowService().execute(generated)
    return generated, result


def test_generated_workflow_is_dag() -> None:
    generated, _ = run_default()
    positions = {task.id: index for index, task in enumerate(generated.workflow.tasks)}
    for dependency in generated.workflow.dependencies:
        assert positions[dependency.source] < positions[dependency.target]


def test_seeded_generation_is_deterministic() -> None:
    request = SimulationRequest(seed=99, task_count=10)
    first = GenerateSimulationService().execute(request)
    second = GenerateSimulationService().execute(request)
    assert first.model_dump() == second.model_dump()


def test_akoflow_yaml_activities_build_workflow_dag() -> None:
    request = SimulationRequest(seed=12, workflow_yaml=AKOFLOW_YAML)
    generated = GenerateSimulationService().execute(request)
    result = ScheduleWorkflowService().execute(generated)

    assert generated.workflow.preset == "wf-synthetic-dag"
    assert [task.id for task in generated.workflow.tasks] == [
        "mprojectid0000001",
        "mprojectid0000002",
        "mdifffitid0000005",
    ]
    assert generated.workflow.predecessor_sets["mdifffitid0000005"] == [
        "mprojectid0000001",
        "mprojectid0000002",
    ]
    assert generated.workflow.tasks[0].workflow_stage == "mProject"
    assert generated.workflow.tasks[2].workflow_stage == "mDiffFit"
    assert generated.workflow.tasks[0].cpu == 1
    assert generated.workflow.tasks[0].memory == 0.25
    assert sorted(assignment.task_id for assignment in result.assignments) == sorted(task.id for task in generated.workflow.tasks)


def test_cluster_resources_are_owned_capacity() -> None:
    request = SimulationRequest(seed=17, task_count=8, cluster_machines=2, cloud_machines=2)
    generated = GenerateSimulationService().execute(request)

    cluster_ids = {resource.id for resource in generated.resources if resource.kind == "cluster"}
    cloud_ids = {resource.id for resource in generated.resources if resource.kind == "cloud"}
    assert cluster_ids
    assert cloud_ids

    for resource in generated.resources:
        if resource.kind == "cluster":
            assert resource.price_per_cpu_second == 0
            assert resource.price_per_gb_second == 0
            assert resource.financial_network_price == 0
        else:
            assert resource.price_per_cpu_second > 0
            assert resource.price_per_gb_second > 0
            assert resource.financial_network_price > 0

    for left in cluster_ids:
        for right in cluster_ids:
            assert generated.matrices.financial_network_cost[left][right] == 0


def test_custom_resource_specs_define_core_capacity_and_bandwidth() -> None:
    request = SimulationRequest(
        seed=7,
        resource_specs=[
            {
                "id": "c1",
                "name": "cluster-large",
                "kind": "cluster",
                "cores": 3,
                "memory": 64,
                "bandwidth": 1200,
                "boot_overhead": 0,
                "location": "on-prem",
            },
            {
                "id": "v1",
                "name": "cloud-fast",
                "kind": "cloud",
                "cores": 4,
                "memory": 96,
                "bandwidth": 500,
                "boot_overhead": 15,
                "location": "eu-west",
            },
        ],
    )

    generated = GenerateSimulationService().execute(request)
    resources = {resource.id: resource for resource in generated.resources}

    assert resources["c1"].name == "cluster-large"
    assert len(resources["c1"].cores) == 3
    assert resources["c1"].cpu == 3
    assert resources["c1"].memory == 64
    assert resources["c1"].bandwidth == 1200
    assert resources["c1"].boot_overhead == 0
    assert resources["v1"].status == "cold"
    assert len(resources["v1"].cores) == 4
    assert resources["v1"].cpu == 4
    assert resources["v1"].boot_overhead == 15
    assert generated.matrices.bandwidth_bw["c1"]["v1"] == 500


def test_cloud_resources_start_cold_with_node_boot_overhead() -> None:
    request = SimulationRequest(seed=17, task_count=8, cluster_machines=2, cloud_machines=3)
    generated = GenerateSimulationService().execute(request)

    for resource in generated.resources:
        if resource.kind == "cloud":
            assert resource.status == "cold"
            assert resource.boot_overhead > 0
        else:
            assert resource.status == "warm"
            assert resource.boot_overhead == 0


def test_cloud_resources_are_eligible_when_feasible() -> None:
    request = SimulationRequest(seed=17, task_count=8, cluster_machines=2, cloud_machines=3)
    generated = GenerateSimulationService().execute(request)
    for resource in generated.resources:
        if resource.kind == "cluster":
            resource.cpu = 0.1
            resource.memory = 0.1
    result = ScheduleWorkflowService().execute(generated)
    resources = {resource.id: resource for resource in result.resources}

    assert any(resources[assignment.resource_id].kind == "cloud" for assignment in result.assignments)


def test_cold_node_uses_resource_boot_overhead() -> None:
    request = SimulationRequest(seed=17, task_count=4, cluster_machines=1, cloud_machines=1)
    generated = GenerateSimulationService().execute(request)
    for resource in generated.resources:
        if resource.kind == "cluster":
            resource.cpu = 0.1
            resource.memory = 0.1
        else:
            resource.status = "cold"
            resource.boot_overhead = 13.5

    result = ScheduleWorkflowService().execute(generated)
    first_cloud_assignment = next(assignment for assignment in result.assignments if assignment.resource_id.startswith("v"))

    assert first_cloud_assignment.boot_overhead == 13.5


def test_cold_node_blocks_all_cores_until_boot_completes() -> None:
    request = SimulationRequest(seed=19, task_count=4, cluster_machines=1, cloud_machines=1, cores_per_machine=2)
    generated = GenerateSimulationService().execute(request)
    generated.workflow.dependencies = []
    generated.workflow.predecessor_sets = {task.id: [] for task in generated.workflow.tasks}
    for task in generated.workflow.tasks:
        task.predecessors = []
        task.successors = []
    for resource in generated.resources:
        if resource.kind == "cluster":
            resource.cpu = 0.1
            resource.memory = 0.1
            continue
        resource.status = "cold"
        resource.boot_overhead = 11.0
        for task in generated.workflow.tasks:
            generated.matrices.et_0[task.id][resource.id] = 5.0
            generated.matrices.et_star[task.id][resource.id] = 5.0
            generated.matrices.container_overhead[task.id][resource.id] = 0.0

    result = ScheduleWorkflowService().execute(generated)
    cloud_assignments = [assignment for assignment in result.assignments if assignment.resource_id.startswith("v")]
    first_booted = next(assignment for assignment in cloud_assignments if assignment.boot_overhead == 11.0)

    assert first_booted.start_time == 11.0
    assert all(assignment.start_time >= 11.0 for assignment in cloud_assignments)
    assert sum(1 for assignment in cloud_assignments if assignment.boot_overhead == 11.0) == 1


def test_cloud_node_stops_when_idle_gap_pays_next_boot() -> None:
    request = SimulationRequest(seed=23, task_count=3, cluster_machines=1, cloud_machines=2, cores_per_machine=1)
    generated = GenerateSimulationService().execute(request)
    tasks = generated.workflow.tasks
    generated.workflow.dependencies = [generated.workflow.dependencies[0].model_copy(update={"source": tasks[1].id, "target": tasks[2].id, "data_mb": 20.0})]
    generated.workflow.predecessor_sets = {
        tasks[0].id: [],
        tasks[1].id: [],
        tasks[2].id: [tasks[1].id],
    }
    tasks[0].predecessors = []
    tasks[0].successors = []
    tasks[1].predecessors = []
    tasks[1].successors = [tasks[2].id]
    tasks[2].predecessors = [tasks[1].id]
    tasks[2].successors = []

    for resource in generated.resources:
        if resource.kind == "cluster":
            resource.cpu = 0.1
            resource.memory = 0.1
            continue
        resource.status = "cold"
        resource.boot_overhead = 5.0
        for task in tasks:
            generated.matrices.container_overhead[task.id][resource.id] = 0.0
            generated.matrices.et_0[task.id][resource.id] = 100.0
            generated.matrices.et_star[task.id][resource.id] = 100.0

    generated.matrices.et_0[tasks[0].id]["v1"] = 1.0
    generated.matrices.et_star[tasks[0].id]["v1"] = 1.0
    generated.matrices.et_0[tasks[1].id]["v2"] = 20.0
    generated.matrices.et_star[tasks[1].id]["v2"] = 20.0
    generated.matrices.et_0[tasks[2].id]["v1"] = 1.0
    generated.matrices.et_star[tasks[2].id]["v1"] = 1.0
    generated.matrices.bandwidth_bw["v2"]["v1"] = 10_000.0

    result = ScheduleWorkflowService().execute(generated)
    assignments = {assignment.task_id: assignment for assignment in result.assignments}
    stopped = result.machine_stop_intervals[0]
    transfer = generated.workflow.dependencies[0].data_mb / generated.matrices.bandwidth_bw["v2"]["v1"]

    assert assignments[tasks[0].id].resource_id == "v1"
    assert assignments[tasks[1].id].resource_id == "v2"
    assert assignments[tasks[2].id].resource_id == "v1"
    assert assignments[tasks[2].id].boot_overhead == 5.0
    assert assignments[tasks[2].id].start_time == round(assignments[tasks[1].id].finish_time + transfer, 3)
    assert stopped.resource_id == "v1"
    assert stopped.stop_time == assignments[tasks[0].id].finish_time
    assert stopped.boot_start_time == round(assignments[tasks[2].id].start_time - 5.0, 3)
    assert stopped.boot_finish_time == assignments[tasks[2].id].start_time


def test_every_task_receives_exactly_one_assignment() -> None:
    generated, result = run_default()
    assigned = [assignment.task_id for assignment in result.assignments]
    assert sorted(assigned) == sorted(task.id for task in generated.workflow.tasks)
    assert len(assigned) == len(set(assigned))


def test_scheduler_steps_report_candidates_and_selected_machine() -> None:
    generated, result = run_default()

    assert len(result.scheduler_steps) == len(generated.workflow.tasks)
    for step in result.scheduler_steps:
        assert step.candidates
        selected = [candidate for candidate in step.candidates if candidate.selected]
        assert len(selected) == 1
        assert selected[0].resource_id == step.selected_resource_id
        assert selected[0].core_id == step.selected_core_id
        assert selected[0].rank == 1
        assert all(candidate.task_id == step.task_id for candidate in step.candidates)


def test_start_time_respects_predecessors_and_resource_availability() -> None:
    generated, result = run_default()
    assignment_by_task = {assignment.task_id: assignment for assignment in result.assignments}
    resource_for_task = {assignment.task_id: assignment.resource_id for assignment in result.assignments}
    previous_on_core = {}

    for assignment in result.assignments:
        assert assignment.start_time >= previous_on_core.get(assignment.core_id, 0.0)
        previous_on_core[assignment.core_id] = assignment.finish_time

        for dependency in generated.workflow.dependencies:
            if dependency.target != assignment.task_id:
                continue
            predecessor = assignment_by_task[dependency.source]
            bandwidth = generated.matrices.bandwidth_bw[resource_for_task[dependency.source]][assignment.resource_id]
            transfer = 0.0 if predecessor.resource_id == assignment.resource_id else dependency.data_mb / bandwidth
            assert assignment.start_time >= predecessor.finish_time + transfer


def test_core_and_memory_infeasible_assignments_are_filtered() -> None:
    generated, result = run_default()
    resources = {resource.id: resource for resource in generated.resources}
    tasks = {task.id: task for task in generated.workflow.tasks}
    for assignment in result.assignments:
        task = tasks[assignment.task_id]
        resource = resources[assignment.resource_id]
        assert task.cpu <= resource.cpu
        assert task.memory <= resource.memory


def test_cost_makespan_and_objective_are_consistent() -> None:
    generated, result = run_default()
    assert result.timing_variables.makespan == max(result.timing_variables.ft.values())
    assert result.cost_variables.b_used == round(sum(result.cost_variables.fc.values()), 4)
    assert result.cost_variables.p_cc == round(sum(result.cost_variables.c_fin.values()), 4)
    assert result.cost_variables.c_w == round(result.cost_variables.b_used + result.cost_variables.p_cc, 4)

    for step in result.scheduler_steps:
        for candidate in step.candidates:
            expected_score = round(
                generated.sla.weight_time * candidate.score.time_score
                + generated.sla.weight_cost * candidate.score.cost_score,
                5,
            )
            assert abs(candidate.score.total_score - expected_score) <= 0.00002


def test_interference_metrics_are_reported_separately_from_objective() -> None:
    generated, result = run_default()
    total_interference = 0.0

    for assignment in result.assignments:
        base_runtime = generated.matrices.et_0[assignment.task_id][assignment.resource_id]
        total_interference += max(0.0, assignment.effective_runtime - base_runtime)

    assert result.interference_variables.total_interference_time == round(total_interference, 3)
    assert result.interference_variables.average_phi_n == round(
        sum(assignment.phi_n for assignment in result.assignments) / len(result.assignments),
        4,
    )


def test_deviation_metrics_match_paper_definitions() -> None:
    generated, result = run_default()

    for assignment in result.assignments:
        task_id = assignment.task_id
        resource_id = assignment.resource_id
        et_0 = generated.matrices.et_0[task_id][resource_id]
        et_obs = result.deviation_variables.et_obs[task_id]
        variance = round(et_obs - et_0, 3)
        excess = round(max(0.0, variance), 3)

        assert result.deviation_variables.var[task_id] == variance
        assert result.deviation_variables.excess[task_id] == excess
        assert result.deviation_variables.d_time[task_id] == round(variance / et_0, 4)
        assert result.deviation_variables.d_excess[task_id] == round(excess / et_0, 4)

    for resource in result.resources:
        values = [
            result.deviation_variables.d_excess[assignment.task_id]
            for assignment in result.assignments
            if assignment.resource_id == resource.id
        ]
        if values:
            assert result.deviation_variables.d_n[resource.id] == round(sum(values) / len(values), 4)

    assert result.deviation_variables.d_w_time == round(
        sum(result.deviation_variables.d_excess.values()) / len(result.deviation_variables.d_excess),
        4,
    )


def test_matrix_dimensions_match_task_and_resource_counts() -> None:
    generated, _ = run_default()
    task_ids = {task.id for task in generated.workflow.tasks}
    resource_ids = {resource.id for resource in generated.resources}

    assert set(generated.matrices.et_0.keys()) == task_ids
    assert set(generated.matrices.container_overhead.keys()) == task_ids
    for values in generated.matrices.et_0.values():
        assert set(values.keys()) == resource_ids
    assert set(generated.matrices.bandwidth_bw.keys()) == resource_ids
    for values in generated.matrices.bandwidth_bw.values():
        assert set(values.keys()) == resource_ids


def test_beam_optimizer_returns_feasible_options_within_budget_and_deadline() -> None:
    request = SimulationRequest(
        seed=42,
        task_count=8,
        edge_density=0.3,
        budget_limit=9999.0,
        deadline_limit=9999.0,
        option_count=5,
    )
    generated = GenerateSimulationService().execute(request)
    response = BeamScheduleOptimizerService().execute(generated)

    assert 1 <= len(response.options) <= request.option_count
    assert response.selected_option_id == response.options[0].id
    for option in response.options:
        assert option.feasible
        assert option.budget_used <= request.budget_limit
        assert option.makespan <= request.deadline_limit
        assert option.budget_violation == 0
        assert option.deadline_violation == 0


def test_beam_optimizer_returns_best_violations_when_no_option_is_feasible() -> None:
    request = SimulationRequest(
        seed=42,
        task_count=8,
        edge_density=0.3,
        budget_limit=0.001,
        deadline_limit=1.0,
        option_count=3,
    )
    generated = GenerateSimulationService().execute(request)
    response = BeamScheduleOptimizerService().execute(generated)

    assert 1 <= len(response.options) <= request.option_count
    assert all(not option.feasible for option in response.options)
    assert all(option.budget_violation > 0 or option.deadline_violation > 0 for option in response.options)


def test_option_count_defaults_and_rejects_values_above_maximum() -> None:
    assert SimulationRequest().option_count == 5
    assert SimulationRequest(option_count=100).option_count == 100
    with pytest.raises(ValidationError):
        SimulationRequest(option_count=101)


def test_beam_options_do_not_share_mutable_matrices_and_are_deduplicated() -> None:
    request = SimulationRequest(
        seed=42,
        task_count=8,
        edge_density=0.3,
        budget_limit=9999.0,
        deadline_limit=9999.0,
        option_count=5,
    )
    generated = GenerateSimulationService().execute(request)
    response = BeamScheduleOptimizerService().execute(generated)

    keys = {(round(option.budget_used, 3), round(option.makespan, 3)) for option in response.options}
    signatures = {option.machine_signature for option in response.options}
    assert len(signatures) == len(response.options)
    assert all("-core-" not in option.machine_signature for option in response.options)
    if len(response.options) < 2:
        return

    first = response.options[0].result
    second = response.options[1].result
    task_id = first.assignments[0].task_id
    resource_id = first.assignments[0].resource_id
    original_second_value = second.matrices.et_star[task_id][resource_id]
    first.matrices.et_star[task_id][resource_id] = -999

    assert second.matrices.et_star[task_id][resource_id] == original_second_value


def test_beam_option_identity_ignores_core_but_preserves_core_for_rendering() -> None:
    request = SimulationRequest(
        seed=42,
        task_count=8,
        edge_density=0.3,
        budget_limit=9999.0,
        deadline_limit=9999.0,
        option_count=20,
    )
    generated = GenerateSimulationService().execute(request)
    response = BeamScheduleOptimizerService().execute(generated)

    semantic_signatures = {
        tuple((assignment.task_id, assignment.resource_id) for assignment in option.result.assignments)
        for option in response.options
    }
    assert len(semantic_signatures) == len(response.options)
    assert all("-core-" not in option.machine_signature for option in response.options)
    assert all(assignment.core_id for option in response.options for assignment in option.result.assignments)


def test_beam_optimizer_returns_machine_diverse_options_for_large_option_count() -> None:
    request = SimulationRequest(
        seed=42,
        task_count=10,
        edge_density=0.25,
        budget_limit=9999.0,
        deadline_limit=9999.0,
        option_count=100,
    )
    generated = GenerateSimulationService().execute(request)
    response = BeamScheduleOptimizerService().execute(generated)

    assert len(response.options) > 5
    assert len({option.machine_signature for option in response.options}) == len(response.options)
    assert all(option.machine_distribution for option in response.options)
    assert all(option.weighted_score >= 0 for option in response.options)


def test_selected_option_is_best_weighted_option() -> None:
    request = SimulationRequest(
        seed=42,
        task_count=10,
        edge_density=0.25,
        budget_limit=9999.0,
        deadline_limit=9999.0,
        option_count=30,
        weight_time=1.0,
        weight_cost=0.0,
    )
    generated = GenerateSimulationService().execute(request)
    response = BeamScheduleOptimizerService().execute(generated)

    selected = next(option for option in response.options if option.id == response.selected_option_id)
    assert selected.recommended
    assert selected.weighted_score == min(option.weighted_score for option in response.options)
