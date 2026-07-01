import Metric from '../Metric.jsx';
import { buildGanttContextByTask } from '../../lib/ganttContext.js';
import { fmt } from '../../lib/format.js';

export default function DetailsPanel({ result, assignment, taskId }) {
  if (!result || !assignment) {
    return <div className="details-empty">Select a task to inspect its schedule.</div>;
  }
  const task = result.workflow.tasks.find((item) => item.id === taskId);
  const resource = result.resources.find((item) => item.id === assignment.resource_id);
  const ganttContext = buildGanttContextByTask(result)[taskId];
  return (
    <div className="details">
      <h2>{task.label}</h2>
      <p>{task.workflow_stage} on {resource.name}</p>
      <Metric label="Start ST" value={fmt(assignment.start_time)} />
      <Metric label="Finish FT" value={fmt(assignment.finish_time)} />
      <Metric label="Effective ET*" value={fmt(assignment.effective_runtime)} />
      <Metric label="Base ET_0" value={fmt(result.matrices.et_0[task.id]?.[assignment.resource_id])} />
      <Metric label="Interference time" value={fmt(Math.max(0, assignment.effective_runtime - (result.matrices.et_0[task.id]?.[assignment.resource_id] ?? assignment.effective_runtime)))} />
      <Metric label="Transfer delay" value={fmt(assignment.transfer_delay)} />
      <Metric label="Boot overhead" value={fmt(assignment.boot_overhead)} />
      <Metric label="Container overhead" value={fmt(assignment.container_overhead)} />
      <Metric label="Phi_n" value={fmt(assignment.phi_n)} />
      <Metric label="C_core" value={fmt(result.cost_variables.c_cpu[task.id])} />
      <Metric label="C_mem" value={fmt(result.cost_variables.c_mem[task.id])} />
      <Metric label="C_fin" value={fmt(result.cost_variables.c_fin[task.id])} />
      <Metric label="FC" value={fmt(result.cost_variables.fc[task.id])} />
      <Metric label="ET_obs" value={fmt(result.deviation_variables.et_obs[task.id])} />
      <Metric label="D_time" value={fmt(result.deviation_variables.d_time[task.id])} />
      <Metric label="D_excess" value={fmt(result.deviation_variables.d_excess[task.id])} />
      <Metric label="D_N" value={fmt(result.deviation_variables.d_n[assignment.resource_id])} />
      {task.runtime && <Metric label="Runtime" value={task.runtime} />}
      <section className="score-box">
        <strong>Score breakdown</strong>
        <span>time {fmt(assignment.score.time_score)}</span>
        <span>cost {fmt(assignment.score.cost_score)}</span>
        <span>interference {fmt(assignment.score.interference_score)}</span>
        <span>total {fmt(assignment.score.total_score)}</span>
      </section>
      <section className="score-box">
        <strong>Predecessors</strong>
        {(task.predecessors.length ? task.predecessors : ["none"]).map((item) => <span key={item}>{item}</span>)}
      </section>
      <section className="score-box">
        <strong>Gantt timing context</strong>
        <span>{ganttContext?.interferenceText || "Interference: none"}</span>
        <span>{ganttContext?.transferText || "Transfer: none"}</span>
        <span>{ganttContext?.containerText || "Container: none"}</span>
        <span>{ganttContext?.bootText || "Boot: none"}</span>
        <span>{ganttContext?.stopText || "Machine stop: none"}</span>
      </section>
      {task.run && (
        <section className="score-box">
          <strong>Command</strong>
          <code>{task.run}</code>
        </section>
      )}
    </div>
  );
}
