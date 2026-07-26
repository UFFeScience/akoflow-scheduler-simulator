#!/usr/bin/env python3
"""Import the WfCommons Montage DSS 2.0° instance for Akoflow simulations."""

from __future__ import annotations

import csv
import hashlib
import json
import shlex
import urllib.request
from pathlib import Path


SOURCE_URL = (
    "https://raw.githubusercontent.com/wfcommons/WfInstances/main/"
    "pegasus/montage/montage-chameleon-dss-20d-001.json"
)
C3D_REFERENCE_GFLOPS = 422.4
FLOPS_PER_CYCLE = 8.0


def yaml_string(value: str) -> str:
    return json.dumps(value, ensure_ascii=False)


def normalized_id(value: str) -> str:
    return value.lower().replace("_", "")


def main() -> None:
    repo_root = Path(__file__).resolve().parents[2]
    experiments = repo_root / "experiments"
    with urllib.request.urlopen(SOURCE_URL) as response:
        source_bytes = response.read()
    instance = json.loads(source_bytes)

    execution = instance["workflow"]["execution"]
    specification = instance["workflow"]["specification"]
    execution_by_id = {task["id"]: task for task in execution["tasks"]}
    specification_by_id = {task["id"]: task for task in specification["tasks"]}
    files = {item["id"]: int(item["sizeInBytes"]) for item in specification["files"]}
    machines = {machine["nodeName"]: machine for machine in execution["machines"]}

    yaml_path = experiments / "wf-montage-chameleon-dss-20d-001.yaml"
    runtime_path = experiments / "montage_chameleon_dss_20d_001_runtimes.csv"
    dependency_path = experiments / "montage_chameleon_dss_20d_001_dependencies.csv"
    provenance_path = experiments / "montage_chameleon_dss_20d_001_provenance.json"

    yaml_lines = [
        "name: wf-montage-chameleon-dss-20d-001",
        "spec:",
        "  storagePolicy:",
        '    storageClassName: "standard-rwo"',
        '    storageSize: "256Mi"',
        "    type: distributed",
        '  runtime: "k8s1"',
        '  mountPath: "/data"',
        '  namespace: "akoflow"',
        "",
        "  image: ovvesley/akoflow-wf-montage:050d",
        "  activities:",
    ]
    runtime_rows = []
    dependency_rows = []

    for spec_task in specification["tasks"]:
        task_id = spec_task["id"]
        execution_task = execution_by_id[task_id]
        normalized = normalized_id(task_id)
        command = execution_task["command"]
        run = " ".join(
            [shlex.quote(command["program"])]
            + [shlex.quote(str(argument)) for argument in command.get("arguments", [])]
        )
        yaml_lines.extend(
            [
                f"    - name: {normalized}",
                f"      run: {yaml_string(run)}",
                "      memoryLimit: 256Mi",
                "      cpuLimit: 1.0",
            ]
        )
        parents = spec_task.get("parents", [])
        if parents:
            yaml_lines.append("      dependsOn:")
            yaml_lines.extend(f"        - {normalized_id(parent)}" for parent in parents)
        yaml_lines.append("")

        source_machine = execution_task["machines"][0]
        cpu = machines[source_machine]["cpu"]
        source_gflops = (
            float(cpu["coreCount"])
            * float(cpu["speedInMHz"])
            / 1000.0
            * FLOPS_PER_CYCLE
        )
        source_speedup = source_gflops / C3D_REFERENCE_GFLOPS
        observed_runtime = float(execution_task["runtimeInSeconds"])
        runtime_rows.append(
            [
                normalized,
                command["program"].lower(),
                source_machine,
                cpu["coreCount"],
                cpu["speedInMHz"],
                f"{source_gflops:.6f}",
                f"{C3D_REFERENCE_GFLOPS:.6f}",
                f"{source_speedup:.9f}",
                f"{observed_runtime:.6f}",
                f"{observed_runtime * source_speedup:.6f}",
                execution_task.get("avgCPU", ""),
            ]
        )

        child_inputs = set(spec_task.get("inputFiles", []))
        for parent_id in parents:
            parent_outputs = set(specification_by_id[parent_id].get("outputFiles", []))
            transferred_files = sorted(parent_outputs & child_inputs)
            transferred_bytes = sum(files[file_id] for file_id in transferred_files)
            dependency_rows.append(
                [
                    normalized_id(parent_id),
                    normalized,
                    f"{transferred_bytes / 1_000_000.0:.9f}",
                    transferred_bytes,
                    "|".join(transferred_files),
                ]
            )

    yaml_path.write_text("\n".join(yaml_lines), encoding="utf-8")
    with runtime_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(
            [
                "activity_id",
                "stage",
                "source_machine",
                "source_cores",
                "source_speed_mhz",
                "source_peak_gflops",
                "c3d_reference_gflops",
                "source_speedup_vs_c3d",
                "observed_runtime_seconds",
                "et0_c3d_seconds",
                "avg_cpu_percent",
            ]
        )
        writer.writerows(runtime_rows)
    with dependency_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(["source", "target", "data_mb", "data_bytes", "files"])
        writer.writerows(dependency_rows)

    provenance = {
        "source_url": SOURCE_URL,
        "source_sha256": hashlib.sha256(source_bytes).hexdigest(),
        "source_schema_version": instance["schemaVersion"],
        "workflow_name": instance["name"],
        "task_count": len(specification["tasks"]),
        "dependency_count": len(dependency_rows),
        "file_count": len(files),
        "observed_makespan_seconds": execution["makespanInSeconds"],
        "normalization": {
            "formula": "peak_gflops = cores * GHz * 8 FLOP/cycle",
            "flops_per_cycle": FLOPS_PER_CYCLE,
            "c3d_standard_16_reference_gflops": C3D_REFERENCE_GFLOPS,
            "et0_formula": "observed_runtime_seconds * source_peak_gflops / 422.4",
        },
        "machines": [
            {
                "node_name": name,
                "cores": machine["cpu"]["coreCount"],
                "speed_mhz": machine["cpu"]["speedInMHz"],
                "peak_gflops": (
                    machine["cpu"]["coreCount"]
                    * machine["cpu"]["speedInMHz"]
                    / 1000.0
                    * FLOPS_PER_CYCLE
                ),
            }
            for name, machine in sorted(machines.items())
        ],
    }
    provenance_path.write_text(
        json.dumps(provenance, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )
    print(
        json.dumps(
            {
                "yaml": str(yaml_path),
                "tasks": len(runtime_rows),
                "dependencies": len(dependency_rows),
                "source_sha256": provenance["source_sha256"],
            }
        )
    )


if __name__ == "__main__":
    main()
