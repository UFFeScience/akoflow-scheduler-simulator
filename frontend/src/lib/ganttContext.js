import { fmt } from './format.js';

export function buildGanttContextByTask(result) {
  const assignmentsByTask = Object.fromEntries(result.assignments.map((assignment) => [assignment.task_id, assignment]));
  const resourcesById = Object.fromEntries(result.resources.map((resource) => [resource.id, resource]));
  const selectedCandidateByTask = Object.fromEntries(
    (result.scheduler_steps || []).map((step) => [step.task_id, step.candidates.find((candidate) => candidate.selected)]),
  );
  return Object.fromEntries(
    result.assignments.map((assignment) => {
      const baseRuntime = result.matrices.et_0[assignment.task_id]?.[assignment.resource_id] ?? assignment.effective_runtime;
      const interferenceRuntime = Math.max(0, assignment.effective_runtime - baseRuntime);
      const selectedCandidate = selectedCandidateByTask[assignment.task_id];
      const pairText = selectedCandidate?.pairwise_interference?.length
        ? selectedCandidate.pairwise_interference
          .map((pair) => `${pair.other_task_id} (${fmt(pair.value)})`)
          .join(", ")
        : "none";
      const transfers = result.workflow.dependencies
        .filter((dependency) => dependency.target === assignment.task_id)
        .map((dependency) => {
          const predecessor = assignmentsByTask[dependency.source];
          if (!predecessor) return null;
          const fromResource = resourcesById[predecessor.resource_id]?.name || predecessor.resource_id;
          const toResource = resourcesById[assignment.resource_id]?.name || assignment.resource_id;
          const bandwidth = result.matrices.bandwidth_bw[predecessor.resource_id]?.[assignment.resource_id];
          const transfer = predecessor.resource_id === assignment.resource_id || !bandwidth ? 0 : dependency.data_mb / bandwidth;
          return `${dependency.source}: ${fromResource} -> ${toResource}, ${fmt(dependency.data_mb)} MB, ${fmt(transfer)}s`;
        })
        .filter(Boolean);
      const resource = resourcesById[assignment.resource_id];
      const stopInterval = (result.machine_stop_intervals || []).find(
        (item) => (
          item.resource_id === assignment.resource_id
          && Math.abs(item.boot_finish_time - (assignment.start_time - assignment.container_overhead)) < 0.001
        ),
      );
      const lines = [
        `${assignment.task_id}`,
        `Base ET_0: ${fmt(baseRuntime)}s`,
        `Interference: +${fmt(interferenceRuntime)}s with ${pairText}`,
        `Container overhead: +${fmt(assignment.container_overhead)}s on ${resource?.name || assignment.resource_id}`,
        `Boot overhead: +${fmt(assignment.boot_overhead)}s (${resource?.status || "unknown"} ${resource?.kind || "machine"}, node boot ${fmt(resource?.boot_overhead)}s)`,
        stopInterval
          ? `Machine stopped: ${fmt(stopInterval.stop_time)}-${fmt(stopInterval.boot_start_time)}s, boot ${fmt(stopInterval.boot_start_time)}-${fmt(stopInterval.boot_finish_time)}s`
          : "Machine stopped: no",
        `Transfer delay: +${fmt(assignment.transfer_delay)}s${transfers.length ? ` from ${transfers.join("; ")}` : " (no cross-machine predecessor transfer)"}`,
      ];
      return [
        assignment.task_id,
        {
          title: lines.join("\n"),
          interferenceText: `Interference: +${fmt(interferenceRuntime)}s with ${pairText}`,
          transferText: `Transfer: +${fmt(assignment.transfer_delay)}s${transfers.length ? ` from ${transfers.join("; ")}` : " (none)"}`,
          containerText: `Container: +${fmt(assignment.container_overhead)}s on ${resource?.name || assignment.resource_id}`,
          bootText: `Boot: +${fmt(assignment.boot_overhead)}s for ${resource?.status || "unknown"} ${resource?.kind || "machine"} (node boot ${fmt(resource?.boot_overhead)}s)`,
          stopText: stopInterval
            ? `Machine stop: ${fmt(stopInterval.stop_time)}-${fmt(stopInterval.boot_start_time)}s, boot paid ${fmt(stopInterval.boot_start_time)}-${fmt(stopInterval.boot_finish_time)}s`
            : "Machine stop: none",
        },
      ];
    }),
  );
}
