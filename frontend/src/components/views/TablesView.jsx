import { fmt } from '../../lib/format.js';

export default function TablesView({ result, onSelect }) {
  return (
    <div className="data-section">
      <h2>Assignments</h2>
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              {["task", "resource", "core", "ST", "FT", "ET*", "transfer", "boot", "container", "Phi_n", "score"].map((heading) => <th key={heading}>{heading}</th>)}
            </tr>
          </thead>
          <tbody>
            {result.assignments.map((item) => (
              <tr key={item.task_id} onClick={() => onSelect(item.task_id)}>
                <td>{item.task_id}</td>
                <td>{item.resource_id}</td>
                <td>{item.core_id}</td>
                <td>{fmt(item.start_time)}</td>
                <td>{fmt(item.finish_time)}</td>
                <td>{fmt(item.effective_runtime)}</td>
                <td>{fmt(item.transfer_delay)}</td>
                <td>{fmt(item.boot_overhead)}</td>
                <td>{fmt(item.container_overhead)}</td>
                <td>{fmt(item.phi_n)}</td>
                <td>{fmt(item.score.total_score)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
