#!/usr/bin/env python3
"""Import one large, real WfInstances execution for each Pegasus application."""

from __future__ import annotations

import csv
import hashlib
import json
import re
import shlex
import urllib.request
from pathlib import Path


BASE_URL = "https://raw.githubusercontent.com/wfcommons/WfInstances/main/pegasus"
C3D_REFERENCE_GFLOPS = 422.4
FLOPS_PER_CYCLE = 8.0
INSTANCES = {
    "wfcommons_1000genome_902": ("1000genome", "1000genome-chameleon-22ch-250k-001.json"),
    "wfcommons_cycles_6543": ("cycles", "cycles-chameleon-10l-3c-12p-001.json"),
    "wfcommons_epigenomics_1695": ("epigenomics", "epigenomics-chameleon-ilmn-6seq-50k-001.json"),
    "wfcommons_seismology_1101": ("seismology", "seismology-chameleon-1100p-001.json"),
    "wfcommons_soykb_676": ("soykb", "soykb-chameleon-50fastq-20ch-001.json"),
    "wfcommons_srasearch_104": ("srasearch", "srasearch-chameleon-50a-002.json"),
}


def normalized_id(value: str) -> str:
    value = re.sub(r"[^A-Za-z0-9-]+", "", value).lower()
    if not value or not value[0].isalpha():
        value = "t" + value
    return value


def import_instance(experiments: Path, workflow_id: str, application: str, filename: str) -> dict:
    source_url = f"{BASE_URL}/{application}/{filename}"
    with urllib.request.urlopen(source_url) as response:
        source_bytes = response.read()
    instance = json.loads(source_bytes)
    execution = instance["workflow"]["execution"]
    specification = instance["workflow"]["specification"]
    execution_by_id = {task["id"]: task for task in execution["tasks"]}
    specification_by_id = {task["id"]: task for task in specification["tasks"]}
    files = {item["id"]: int(item.get("sizeInBytes", 0)) for item in specification.get("files", [])}
    machines = {machine["nodeName"]: machine for machine in execution["machines"]}
    normalized = {task["id"]: normalized_id(task["id"]) for task in specification["tasks"]}
    if len(set(normalized.values())) != len(normalized):
        raise ValueError(f"normalized task ID collision in {workflow_id}")

    stem = workflow_id.replace("wfcommons_", "wfcommons-").replace("_", "-")
    yaml_path = experiments / f"wf-{stem}.yaml"
    runtime_path = experiments / f"{workflow_id}_runtimes.csv"
    dependency_path = experiments / f"{workflow_id}_dependencies.csv"
    provenance_path = experiments / f"{workflow_id}_provenance.json"
    yaml_lines = [
        f"name: {stem}", "spec:", '  runtime: "k8s1"', '  namespace: "akoflow"',
        f"  image: wfcommons/{application}:latest", "  activities:",
    ]
    runtime_rows, dependency_rows = [], []
    for spec_task in specification["tasks"]:
        task_id = spec_task["id"]
        execution_task = execution_by_id[task_id]
        command = execution_task.get("command") or {}
        program = str(command.get("program") or task_id.split("_")[0])
        arguments = command.get("arguments") or []
        run = " ".join([shlex.quote(program)] + [shlex.quote(str(item)) for item in arguments])
        yaml_lines.extend([
            f"    - name: {normalized[task_id]}", f"      run: {json.dumps(run)}",
            "      memoryLimit: 256Mi", "      cpuLimit: 1.0",
        ])
        parents = spec_task.get("parents", [])
        if parents:
            yaml_lines.append("      dependsOn:")
            yaml_lines.extend(f"        - {normalized[parent]}" for parent in parents)
        yaml_lines.append("")

        source_machine = execution_task["machines"][0]
        cpu = machines[source_machine]["cpu"]
        source_gflops = float(cpu["coreCount"]) * float(cpu["speedInMHz"]) / 1000 * FLOPS_PER_CYCLE
        source_speedup = source_gflops / C3D_REFERENCE_GFLOPS
        observed_runtime = float(execution_task["runtimeInSeconds"])
        runtime_rows.append([
            normalized[task_id], program.lower(), source_machine, cpu["coreCount"], cpu["speedInMHz"],
            f"{source_gflops:.6f}", f"{C3D_REFERENCE_GFLOPS:.6f}", f"{source_speedup:.9f}",
            f"{observed_runtime:.6f}", f"{observed_runtime * source_speedup:.6f}", execution_task.get("avgCPU", ""),
        ])
        child_inputs = set(spec_task.get("inputFiles", []))
        for parent_id in parents:
            parent_outputs = set(specification_by_id[parent_id].get("outputFiles", []))
            transferred_files = sorted(parent_outputs & child_inputs)
            transferred_bytes = sum(files.get(file_id, 0) for file_id in transferred_files)
            dependency_rows.append([
                normalized[parent_id], normalized[task_id], f"{transferred_bytes / 1_000_000:.9f}",
                transferred_bytes, "|".join(transferred_files),
            ])

    yaml_path.write_text("\n".join(yaml_lines), encoding="utf-8")
    with runtime_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(["activity_id", "stage", "source_machine", "source_cores", "source_speed_mhz",
                         "source_peak_gflops", "c3d_reference_gflops", "source_speedup_vs_c3d",
                         "observed_runtime_seconds", "et0_c3d_seconds", "avg_cpu_percent"])
        writer.writerows(runtime_rows)
    with dependency_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(["source", "target", "data_mb", "data_bytes", "files"])
        writer.writerows(dependency_rows)
    provenance = {
        "workflow_id": workflow_id, "application": application, "source_url": source_url,
        "source_sha256": hashlib.sha256(source_bytes).hexdigest(), "source_schema_version": instance["schemaVersion"],
        "workflow_name": instance["name"], "task_count": len(runtime_rows),
        "dependency_count": len(dependency_rows), "file_count": len(files),
        "observed_makespan_seconds": execution["makespanInSeconds"],
        "normalization": {"formula": "peak_gflops = cores * GHz * 8 FLOP/cycle",
                          "c3d_standard_16_reference_gflops": C3D_REFERENCE_GFLOPS,
                          "et0_formula": "observed_runtime_seconds * source_peak_gflops / 422.4"},
    }
    provenance_path.write_text(json.dumps(provenance, indent=2) + "\n", encoding="utf-8")
    return provenance


def main() -> None:
    experiments = Path(__file__).resolve().parents[1]
    summary = [import_instance(experiments, workflow_id, *source) for workflow_id, source in INSTANCES.items()]
    print(json.dumps(summary, indent=2))


if __name__ == "__main__":
    main()
