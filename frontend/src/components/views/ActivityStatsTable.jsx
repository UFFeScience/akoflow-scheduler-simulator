import { fmt } from '../../lib/format.js';

export default function ActivityStatsTable({ result, assignmentByTask, stepByTask, onSelect }) {
  return (
    <section className="data-section">
      <h2>Activity statistics</h2>
      <div className="table-scroll">
        <table>
          <thead><tr>{["activity", "stage", "machine", "core", "rank", "candidates", "ET_0", "ET*", "interference", "phi", "FC", "score"].map((heading) => <th key={heading}>{heading}</th>)}</tr></thead>
          <tbody>
            {result.workflow.tasks.map((task) => {
              const assignment = assignmentByTask[task.id];
              const step = stepByTask[task.id];
              const selected = step?.candidates.find((candidate) => candidate.selected);
              return (
                <tr key={task.id} onClick={() => onSelect(task.id)}>
                  <td>{task.id}</td><td>{task.workflow_stage}</td><td>{assignment?.resource_id}</td><td>{assignment?.core_id}</td><td>{selected?.rank}</td><td>{step?.candidates.length}</td><td>{fmt(selected?.base_runtime)}</td><td>{fmt(selected?.effective_runtime)}</td><td>{fmt(selected?.interference_time)}</td><td>{fmt(assignment?.phi_n)}</td><td>{fmt(result.cost_variables.fc[task.id])}</td><td>{fmt(assignment?.score.total_score)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}
