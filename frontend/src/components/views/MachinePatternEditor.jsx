import ControlInput from '../controls/ControlInput.jsx';

export default function MachinePatternEditor({ resources, onChange, onReset }) {
  return (
    <section className="machine-editor">
      <header>
        <div>
          <h2>Machine pattern</h2>
          <p>Defaults are generated from cluster/cloud counts. Edit any machine before matrices are generated.</p>
        </div>
        <button className="secondary-button" type="button" onClick={onReset}>
          Reset pattern
        </button>
      </header>
      <div className="resource-table-wrap">
        <table className="resource-editor-table">
          <thead>
            <tr>
              {["machine", "kind", "cores", "memory GB", "bandwidth MB/s", "boot s", "location"].map((heading) => <th key={heading}>{heading}</th>)}
            </tr>
          </thead>
          <tbody>
            {resources.map((resource) => (
              <tr key={resource.id}>
                <td>
                  <input value={resource.name} onChange={(event) => onChange(resource.id, "name", event.target.value)} />
                  <span>{resource.id}</span>
                </td>
                <td><span className={`status-badge ${resource.kind}`}>{resource.kind}</span></td>
                <td><input type="number" min="1" max="64" step="1" value={resource.cores} onChange={(event) => onChange(resource.id, "cores", Number(event.target.value))} /></td>
                <td><input type="number" min="0.1" step="0.1" value={resource.memory} onChange={(event) => onChange(resource.id, "memory", Number(event.target.value))} /></td>
                <td><input type="number" min="1" step="10" value={resource.bandwidth} onChange={(event) => onChange(resource.id, "bandwidth", Number(event.target.value))} /></td>
                <td><input type="number" min="0" step="0.1" value={resource.boot_overhead} disabled={resource.kind === "cluster"} onChange={(event) => onChange(resource.id, "boot_overhead", Number(event.target.value))} /></td>
                <td><input value={resource.location} onChange={(event) => onChange(resource.id, "location", event.target.value)} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
