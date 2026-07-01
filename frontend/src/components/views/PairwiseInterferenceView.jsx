import { useState } from 'react';
import Metric from '../Metric.jsx';
import { interferenceDimensions } from '../../lib/interference.js';
import { fmt } from '../../lib/format.js';

export default function PairwiseInterferenceView({ result, onSelect }) {
  const [sourceTaskId, setSourceTaskId] = useState(result.workflow.tasks[0]?.id || "");
  const [resourceFilter, setResourceFilter] = useState("all");
  const taskIds = result.workflow.tasks.map((task) => task.id);
  const rows = result.resources
    .filter((resource) => resourceFilter === "all" || resource.id === resourceFilter)
    .flatMap((resource) => {
      const resourceMatrix = result.matrices.interference_i_n[resource.id] || {};
      return taskIds
        .filter((targetTaskId) => targetTaskId !== sourceTaskId)
        .map((targetTaskId) => {
          const dimensions = {
            cpu: resourceMatrix.cpu?.[sourceTaskId]?.[targetTaskId] ?? 0,
            memory: resourceMatrix.memory?.[sourceTaskId]?.[targetTaskId] ?? 0,
            io: resourceMatrix.io?.[sourceTaskId]?.[targetTaskId] ?? 0,
            network: resourceMatrix.network?.[sourceTaskId]?.[targetTaskId] ?? 0,
          };
          const average = (dimensions.cpu + dimensions.memory + dimensions.io + dimensions.network) / 4;
          return {
            resource,
            sourceTaskId,
            targetTaskId,
            dimensions,
            average,
          };
        });
    })
    .sort((left, right) => right.average - left.average || left.resource.id.localeCompare(right.resource.id) || left.targetTaskId.localeCompare(right.targetTaskId));

  return (
    <div className="steps-view">
      <section className="data-section pairwise-toolbar">
        <div>
          <h2>Activity pair interference by machine</h2>
          <p>Rows show Activity A x Activity B on each machine.</p>
        </div>
        <div className="filter-row">
          <label className="control compact-control">
            <span>Activity A</span>
            <select value={sourceTaskId} onChange={(event) => setSourceTaskId(event.target.value)}>
              {taskIds.map((taskId) => <option key={taskId} value={taskId}>{taskId}</option>)}
            </select>
          </label>
          <label className="control compact-control">
            <span>Machine</span>
            <select value={resourceFilter} onChange={(event) => setResourceFilter(event.target.value)}>
              <option value="all">All machines</option>
              {result.resources.map((resource) => <option key={resource.id} value={resource.id}>{resource.name}</option>)}
            </select>
          </label>
        </div>
      </section>

      <div className="stats-grid">
        <Metric label="Pairs shown" value={rows.length} />
        <Metric label="Machines" value={resourceFilter === "all" ? result.resources.length : 1} />
        <Metric label="Max average" value={fmt(rows[0]?.average)} />
        <Metric label="Source activity" value={sourceTaskId} />
      </div>

      <section className="data-section">
        <h2>Pairwise matrix rows</h2>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                {["machine", "kind", "activity A", "activity B", "average", "core", "memory", "io", "network"].map((heading) => <th key={heading}>{heading}</th>)}
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={`${row.resource.id}-${row.sourceTaskId}-${row.targetTaskId}`} onClick={() => onSelect(row.sourceTaskId)}>
                  <td>{row.resource.name}</td>
                  <td>{row.resource.kind}</td>
                  <td>{row.sourceTaskId}</td>
                  <td>{row.targetTaskId}</td>
                  <td>{fmt(row.average)}</td>
                  <td>{fmt(row.dimensions.cpu)}</td>
                  <td>{fmt(row.dimensions.memory)}</td>
                  <td>{fmt(row.dimensions.io)}</td>
                  <td>{fmt(row.dimensions.network)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
