import { fmt } from '../../lib/format.js';

export default function NodeDistributionHeatmap({ machineDistribution, totalRuntime, maxima }) {
  const rows = [
    { key: "activities", label: "Activities", format: (item) => item.count, intensity: (item) => item.count / maxima.count },
    { key: "total-runtime", label: "Total runtime", format: (item) => `${fmt(item.totalRuntime)}s`, intensity: (item) => item.totalRuntime / maxima.runtime },
    { key: "average-runtime", label: "Avg runtime", format: (item) => `${fmt(item.averageRuntime)}s`, intensity: (item) => item.averageRuntime / maxima.average },
    { key: "runtime-share", label: "Runtime share", format: (item) => `${fmt((totalRuntime === 0 ? 0 : item.totalRuntime / totalRuntime) * 100)}%`, intensity: (item) => (totalRuntime === 0 ? 0 : item.totalRuntime / totalRuntime) / maxima.share },
  ];
  return (
    <section className="data-section">
      <h2>Node distribution heatmap</h2>
      <div className="node-heatmap-wrap">
        <table className="node-heatmap">
          <thead><tr><th>Metric</th>{machineDistribution.map(({ resource }) => <th className={`node-heatmap-machine ${resource.kind}`} key={resource.id}><strong>{resource.name}</strong><span>{resource.kind === "cluster" ? "HPC" : "cloud"} / {resource.id}</span></th>)}</tr></thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.key}>
                <th>{row.label}</th>
                {machineDistribution.map((item) => (
                  <td key={`${row.key}-${item.resource.id}`}>
                    <div className={`node-heatmap-cell ${item.resource.kind}`} style={{ "--heatmap-intensity": Math.max(0.08, row.intensity(item)) }} title={`${item.resource.name} / ${row.label}: ${row.format(item)}`}>
                      <strong>{row.format(item)}</strong>
                    </div>
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
