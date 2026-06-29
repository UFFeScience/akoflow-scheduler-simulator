from __future__ import annotations

import random
import re
from typing import Any, Dict, List, Tuple

import yaml

from app.models import (
    Core,
    Dependency,
    GeneratedSimulation,
    Matrices,
    Resource,
    SLA,
    SimulationRequest,
    Task,
    Workflow,
)


PRESETS = [
    {"id": "Montage", "label": "Montage", "stages": ["mProject", "mDiffFit", "mConcatFit", "mBgModel", "mAdd"]},
    {"id": "CyberShake", "label": "CyberShake", "stages": ["ExtractSGT", "Seismogram", "PeakValCalc", "ZipPSA"]},
    {"id": "Epigenomics", "label": "Epigenomics", "stages": ["FastQSplit", "FilterContams", "Sol2Sanger", "Map", "Merge"]},
    {"id": "Random", "label": "Random DAG", "stages": ["ingest", "transform", "analyze", "reduce", "publish"]},
]


class GenerateWorkflowService:
    def execute(self, request: SimulationRequest) -> Workflow:
        if request.workflow_yaml and request.workflow_yaml.strip():
            return self._from_akoflow_yaml(request)
        rng = random.Random(request.seed)
        preset = next((p for p in PRESETS if p["id"] == request.preset), PRESETS[0])
        tasks: List[Task] = []
        dependencies: List[Dependency] = []

        for index in range(request.task_count):
            stage = preset["stages"][index % len(preset["stages"])]
            tasks.append(
                Task(
                    id=f"t{index + 1}",
                    label=f"{stage}-{index + 1}",
                    workflow_stage=stage,
                    cpu=round(rng.uniform(0.7, 3.4), 2),
                    memory=round(rng.uniform(1.0, 8.0), 2),
                    base_runtime=round(rng.uniform(8.0, 35.0), 2),
                    image=f"{stage.lower()}:latest",
                    predecessors=[],
                )
            )

        for index in range(1, request.task_count):
            candidates = list(range(index))
            rng.shuffle(candidates)
            required_parent = candidates[0]
            edge_sources = {required_parent}
            for candidate in candidates[1:]:
                distance_factor = 1.0 / max(1, index - candidate)
                if rng.random() < request.edge_density * (0.35 + distance_factor):
                    edge_sources.add(candidate)
            for source_index in sorted(edge_sources):
                source = tasks[source_index]
                target = tasks[index]
                target.predecessors.append(source.id)
                source.successors.append(target.id)
                dependencies.append(
                    Dependency(source=source.id, target=target.id, data_mb=round(rng.uniform(20.0, 900.0), 2))
                )

        predecessor_sets = {task.id: sorted(task.predecessors) for task in tasks}
        return Workflow(
            preset=preset["id"],
            tasks=tasks,
            dependencies=dependencies,
            predecessor_sets=predecessor_sets,
        )

    def _from_akoflow_yaml(self, request: SimulationRequest) -> Workflow:
        try:
            document = yaml.safe_load(request.workflow_yaml or "")
        except yaml.YAMLError as exc:
            raise ValueError(f"Invalid workflow YAML: {exc}") from exc
        if not isinstance(document, dict):
            raise ValueError("Workflow YAML must contain a mapping at the root")

        spec = document.get("spec")
        if not isinstance(spec, dict):
            raise ValueError("Workflow YAML must contain spec")
        activities = spec.get("activities")
        if not isinstance(activities, list) or not activities:
            raise ValueError("Workflow YAML must contain spec.activities")

        rng = random.Random(request.seed)
        workflow_name = str(document.get("name") or request.preset or "Akoflow")
        global_runtime = spec.get("runtime")
        global_image = spec.get("image")
        tasks: List[Task] = []
        dependencies: List[Dependency] = []
        seen: set[str] = set()

        for index, activity in enumerate(activities):
            if not isinstance(activity, dict):
                raise ValueError(f"Activity at index {index} must be a mapping")
            name = activity.get("name")
            if not isinstance(name, str) or not name.strip():
                raise ValueError(f"Activity at index {index} must define name")
            task_id = name.strip()
            if task_id in seen:
                raise ValueError(f"Duplicate activity name: {task_id}")
            seen.add(task_id)

            run_command = str(activity.get("run") or "")
            runtime = str(activity.get("runtime") or global_runtime or "")
            stage = self._activity_stage(task_id, run_command)
            cpu = self._parse_cpu_limit(activity.get("cpuLimit"), rng)
            memory = self._parse_memory_limit(activity.get("memoryLimit"), rng)
            base_runtime = round(rng.uniform(8.0, 35.0), 2)
            tasks.append(
                Task(
                    id=task_id,
                    label=task_id,
                    workflow_stage=stage,
                    cpu=cpu,
                    memory=memory,
                    base_runtime=base_runtime,
                    image=str(activity.get("image") or global_image or f"{stage.lower()}:latest"),
                    runtime=runtime or None,
                    run=run_command or None,
                    predecessors=[],
                )
            )

        task_by_id = {task.id: task for task in tasks}
        for activity in activities:
            task_id = str(activity["name"]).strip()
            depends_on = activity.get("dependsOn", [])
            if depends_on is None:
                depends_on = []
            if isinstance(depends_on, str):
                depends_on = [depends_on]
            if not isinstance(depends_on, list):
                raise ValueError(f"dependsOn for {task_id} must be a list")

            for dependency_name in depends_on:
                source_id = str(dependency_name).strip()
                if source_id not in task_by_id:
                    raise ValueError(f"Activity {task_id} depends on unknown activity {source_id}")
                source = task_by_id[source_id]
                target = task_by_id[task_id]
                target.predecessors.append(source.id)
                source.successors.append(target.id)
                dependencies.append(
                    Dependency(source=source.id, target=target.id, data_mb=round(rng.uniform(20.0, 900.0), 2))
                )

        predecessor_sets = {task.id: sorted(task.predecessors) for task in tasks}
        self._validate_dag(tasks, predecessor_sets)
        return Workflow(
            preset=workflow_name,
            tasks=tasks,
            dependencies=dependencies,
            predecessor_sets=predecessor_sets,
        )

    def _activity_stage(self, name: str, run_command: str) -> str:
        for command in re.findall(r"(?:^|&&\s*)([A-Za-z][A-Za-z0-9_-]*)", run_command):
            if command not in {"cd", "cp", "mkdir", "mv", "rm", "set"}:
                return command
        for token in re.split(r"[^A-Za-z]+", name):
            if token:
                return token
        return "activity"

    def _parse_cpu_limit(self, value: Any, rng: random.Random) -> float:
        if isinstance(value, (int, float)):
            return max(0.1, round(float(value), 2))
        if isinstance(value, str):
            match = re.search(r"\d+(?:\.\d+)?", value)
            if match:
                return max(0.1, round(float(match.group(0)), 2))
        return round(rng.uniform(0.7, 3.4), 2)

    def _parse_memory_limit(self, value: Any, rng: random.Random) -> float:
        if isinstance(value, (int, float)):
            return max(0.1, round(float(value) / 1024, 2))
        if isinstance(value, str):
            match = re.search(r"\d+(?:\.\d+)?", value)
            if match:
                amount = float(match.group(0))
                unit = value[match.end() :].strip().lower()
                if unit.startswith("g"):
                    return max(0.1, round(amount, 2))
                return max(0.1, round(amount / 1024, 2))
        return round(rng.uniform(1.0, 8.0), 2)

    def _validate_dag(self, tasks: List[Task], predecessor_sets: Dict[str, List[str]]) -> None:
        remaining = {task.id for task in tasks}
        ordered: List[str] = []
        while remaining:
            ready = sorted(task_id for task_id in remaining if set(predecessor_sets[task_id]).issubset(ordered))
            if not ready:
                raise ValueError("Workflow YAML contains a dependency cycle")
            for task_id in ready:
                ordered.append(task_id)
                remaining.remove(task_id)


class GenerateResourcesService:
    def execute(self, request: SimulationRequest) -> Tuple[List[Resource], Dict[str, Dict[str, float]]]:
        rng = random.Random(request.seed + 11)
        resources: List[Resource] = []
        locations = ["eu-west", "us-east", "ap-south", "on-prem"]

        def make_resource(kind: str, index: int) -> Resource:
            node_id = f"{'c' if kind == 'cluster' else 'v'}{index}"
            cpu_base = 4.0 if kind == "cluster" else 5.5
            mem_base = 16.0 if kind == "cluster" else 24.0
            price_multiplier = 0.0 if kind == "cluster" else 1.0
            stages = PRESETS[index % len(PRESETS)]["stages"]
            return Resource(
                id=node_id,
                name=f"{kind}-{index}",
                kind=kind,  # type: ignore[arg-type]
                cores=[Core(id=f"{node_id}-core-{core + 1}", index=core) for core in range(request.cores_per_machine)],
                cpu=round(cpu_base + rng.uniform(0, 5), 2),
                memory=round(mem_base + rng.uniform(0, 32), 2),
                price_per_cpu_second=round(rng.uniform(0.006, 0.025) * price_multiplier, 5),
                price_per_gb_second=round(rng.uniform(0.001, 0.006) * price_multiplier, 5),
                financial_network_price=0.0 if kind == "cluster" else round(rng.uniform(0.0008, 0.006), 5),
                location=locations[(index + (0 if kind == "cluster" else 1)) % len(locations)],
                status="warm" if kind == "cluster" or rng.random() > 0.4 else "cold",
                image_cache=[f"{stage.lower()}:latest" for stage in stages[: rng.randint(1, len(stages))]],
            )

        for index in range(1, request.cluster_machines + 1):
            resources.append(make_resource("cluster", index))
        for index in range(1, request.cloud_machines + 1):
            resources.append(make_resource("cloud", index))

        bandwidth: Dict[str, Dict[str, float]] = {}
        for left in resources:
            bandwidth[left.id] = {}
            for right in resources:
                same_location = left.location == right.location
                bandwidth[left.id][right.id] = 10_000.0 if left.id == right.id else round(rng.uniform(80, 950) * (2 if same_location else 1), 2)
        return resources, bandwidth


class GenerateInterferenceMatrixService:
    def execute(self, request: SimulationRequest, workflow: Workflow, resources: List[Resource]) -> Dict[str, Dict[str, Dict[str, Dict[str, float]]]]:
        rng = random.Random(request.seed + 23)
        matrix: Dict[str, Dict[str, Dict[str, Dict[str, float]]]] = {}
        dimensions = ["cpu", "memory", "io", "network"]
        for resource in resources:
            matrix[resource.id] = {}
            for dimension in dimensions:
                matrix[resource.id][dimension] = {}
                for source in workflow.tasks:
                    matrix[resource.id][dimension][source.id] = {}
                    for target in workflow.tasks:
                        value = 0.0 if source.id == target.id else rng.uniform(0.0, 0.18)
                        matrix[resource.id][dimension][source.id][target.id] = round(value, 4)
        return matrix


class GenerateSimulationService:
    def __init__(self) -> None:
        self.workflow_service = GenerateWorkflowService()
        self.resources_service = GenerateResourcesService()
        self.interference_service = GenerateInterferenceMatrixService()

    def execute(self, request: SimulationRequest) -> GeneratedSimulation:
        workflow = self.workflow_service.execute(request)
        resources, bandwidth = self.resources_service.execute(request)
        interference = self.interference_service.execute(request, workflow, resources)
        rng = random.Random(request.seed + 37)

        et_0: Dict[str, Dict[str, float]] = {}
        container_overhead: Dict[str, Dict[str, float]] = {}
        for task in workflow.tasks:
            et_0[task.id] = {}
            container_overhead[task.id] = {}
            for resource in resources:
                image_hit = task.image in resource.image_cache
                speed = max(0.35, resource.cpu / max(task.cpu, 0.1))
                et_0[task.id][resource.id] = round(task.base_runtime / speed + rng.uniform(0.0, 4.0), 3)
                container_overhead[task.id][resource.id] = round((0.4 if image_hit else 3.5) + rng.uniform(0.0, 1.8), 3)

        transfer_delay = {left.id: {} for left in resources}
        financial_network_cost = {left.id: {} for left in resources}
        for left in resources:
            for right in resources:
                transfer_delay[left.id][right.id] = 0.0 if left.id == right.id else round(100.0 / bandwidth[left.id][right.id], 4)
                if left.kind == "cluster" and right.kind == "cluster":
                    financial_network_cost[left.id][right.id] = 0.0
                else:
                    financial_network_cost[left.id][right.id] = 0.0 if left.id == right.id else round((left.financial_network_price + right.financial_network_price) / 2, 5)

        matrices = Matrices(
            et_0=et_0,
            et_star={task.id: {resource.id: et_0[task.id][resource.id] for resource in resources} for task in workflow.tasks},
            interference_i_n=interference,
            bandwidth_bw=bandwidth,
            transfer_delay=transfer_delay,
            financial_network_cost=financial_network_cost,
            container_overhead=container_overhead,
        )
        sla = SLA(
            deadline=request.deadline,
            budget=request.budget,
            weight_time=request.weight_time,
            weight_cost=request.weight_cost,
            weight_interference=request.weight_interference,
            penalty_deadline=request.penalty_deadline,
            penalty_budget=request.penalty_budget,
        )
        return GeneratedSimulation(
            id=f"sim-{request.seed}-{len(workflow.tasks)}-{request.cluster_machines}-{request.cloud_machines}",
            seed=request.seed,
            workflow=workflow,
            resources=resources,
            sla=sla,
            matrices=matrices,
        )
