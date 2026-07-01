export function buildActivityStats(result) {
  const stepByTask = Object.fromEntries((result.scheduler_steps || []).map((step) => [step.task_id, step]));
  const assignmentByTask = Object.fromEntries(result.assignments.map((assignment) => [assignment.task_id, assignment]));
  const machineUse = result.resources.map((resource) => ({
    resource,
    assignments: result.assignments
      .filter((assignment) => assignment.resource_id === resource.id)
      .sort((left, right) => left.start_time - right.start_time || left.task_id.localeCompare(right.task_id)),
  }));
  const machineDistribution = machineUse.map(({ resource, assignments }) => {
    const totalRuntime = assignments.reduce((sum, assignment) => sum + assignment.effective_runtime, 0);
    return { resource, assignments, count: assignments.length, totalRuntime, averageRuntime: totalRuntime / Math.max(assignments.length, 1) };
  });
  const totalRuntime = machineDistribution.reduce((sum, item) => sum + item.totalRuntime, 0);
  const maxima = {
    runtime: Math.max(...machineDistribution.map((item) => item.totalRuntime), 1),
    count: Math.max(...machineDistribution.map((item) => item.count), 1),
    average: Math.max(...machineDistribution.map((item) => item.averageRuntime), 1),
    share: Math.max(...machineDistribution.map((item) => (totalRuntime === 0 ? 0 : item.totalRuntime / totalRuntime)), 0.001),
  };
  const makespan = result.scheduler_variables.makespan || 0;
  const bucketSize = Math.max(1, Math.ceil(makespan / 80));
  const timeBuckets = Array.from({ length: Math.max(1, Math.ceil(makespan / bucketSize)) }, (_, index) => ({
    index,
    start: index * bucketSize,
    finish: Math.min((index + 1) * bucketSize, Math.max(makespan, bucketSize)),
  }));
  const nodeTimeCells = machineDistribution.filter((item) => item.count > 0).map((item) => ({
    ...item,
    buckets: timeBuckets.map((bucket) => {
      const running = item.assignments.filter((assignment) => assignment.start_time < bucket.finish && assignment.finish_time > bucket.start);
      return { ...bucket, running, count: running.length };
    }),
  }));
  return { stepByTask, assignmentByTask, machineUse, machineDistribution, totalRuntime, maxima, timeBuckets, nodeTimeCells };
}
