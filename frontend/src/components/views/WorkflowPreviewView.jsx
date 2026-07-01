import DagView from './DagView.jsx';
import { fmt } from '../../lib/format.js';

export default function WorkflowPreviewView({ generated, selectedTaskId, onSelect }) {
  return (
    <div className="steps-view">
      <section className="data-section step-header">
        <div>
          <span>Step 2</span>
          <h2>{generated.workflow.preset}</h2>
          <p>{generated.workflow.tasks.length} activities and {generated.workflow.dependencies.length} dependencies generated.</p>
        </div>
      </section>
      <DagView result={generated} selectedTaskId={selectedTaskId} onSelect={onSelect} />
      <section className="data-section">
        <h2>Dependencies</h2>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>source</th>
                <th>target</th>
                <th>data MB</th>
              </tr>
            </thead>
            <tbody>
              {generated.workflow.dependencies.map((dependency) => (
                <tr key={`${dependency.source}-${dependency.target}`}>
                  <td>{dependency.source}</td>
                  <td>{dependency.target}</td>
                  <td>{fmt(dependency.data_mb)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
