from __future__ import annotations

from app.models import SimulationRequest
from app.services.generation_services import GenerateSimulationService
from app.services.scheduling_services import ScheduleWorkflowService


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


def test_zero_budget_request_is_valid() -> None:
    request = SimulationRequest(budget=0)
    assert request.budget == 0


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
    request = SimulationRequest(seed=17, task_count=8, budget=0, cluster_machines=2, cloud_machines=2)
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


def test_zero_budget_filters_cloud_assignments_and_keeps_costs_zero() -> None:
    request = SimulationRequest(seed=17, task_count=8, budget=0, cluster_machines=3, cloud_machines=3)
    generated = GenerateSimulationService().execute(request)
    result = ScheduleWorkflowService().execute(generated)
    resources = {resource.id: resource for resource in result.resources}

    assert result.assignments
    assert all(resources[assignment.resource_id].kind == "cluster" for assignment in result.assignments)
    assert all(value == 0 for value in result.cost_variables.c_cpu.values())
    assert all(value == 0 for value in result.cost_variables.c_mem.values())
    assert all(value == 0 for value in result.cost_variables.c_fin.values())
    assert result.cost_variables.b_used == 0
    assert result.cost_variables.p_bud_w == 0
    assert result.cost_variables.c_w == result.cost_variables.p_dl_w


def test_positive_budget_keeps_cloud_resources_eligible() -> None:
    request = SimulationRequest(seed=17, task_count=8, budget=260, cluster_machines=2, cloud_machines=3)
    generated = GenerateSimulationService().execute(request)
    for resource in generated.resources:
        if resource.kind == "cluster":
            resource.cpu = 0.1
            resource.memory = 0.1
    result = ScheduleWorkflowService().execute(generated)
    resources = {resource.id: resource for resource in result.resources}

    assert any(resources[assignment.resource_id].kind == "cloud" for assignment in result.assignments)


def test_cold_node_uses_resource_boot_overhead() -> None:
    request = SimulationRequest(seed=17, task_count=4, budget=260, cluster_machines=1, cloud_machines=1)
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
    request = SimulationRequest(seed=19, task_count=4, budget=260, cluster_machines=1, cloud_machines=1, cores_per_machine=2)
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


def test_cost_budget_makespan_and_penalties_are_consistent() -> None:
    _, result = run_default()
    assert result.timing_variables.makespan == max(result.timing_variables.ft.values())
    assert result.cost_variables.b_used == round(sum(result.cost_variables.fc.values()), 4)
    expected_deadline_penalty = round(
        max(0.0, result.timing_variables.makespan - result.sla.deadline) * result.sla.penalty_deadline,
        4,
    )
    expected_budget_penalty = round(
        max(0.0, result.cost_variables.b_used - result.sla.budget) * result.sla.penalty_budget,
        4,
    )
    assert result.cost_variables.p_dl_w == expected_deadline_penalty
    assert result.cost_variables.p_bud_w == expected_budget_penalty


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
