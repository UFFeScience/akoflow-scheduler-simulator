import { useState } from 'react';
import { aggregateInterferenceDimension, buildAggregatedInterferenceMatrix, interferenceDimensionOptions } from '../../lib/interference.js';
import { dimensionLabel } from '../../lib/format.js';
import MatrixTable from './MatrixTable.jsx';

export default function MatricesView({ result }) {
  const [interferenceResourceId, setInterferenceResourceId] = useState(result.resources[0]?.id || "");
  const [interferenceDimension, setInterferenceDimension] = useState("cpu");
  const isAggregateInterference = interferenceDimension === aggregateInterferenceDimension;
  const interferenceMatrix = isAggregateInterference
    ? buildAggregatedInterferenceMatrix(result.matrices.interference_i_n, interferenceResourceId)
    : result.matrices.interference_i_n[interferenceResourceId]?.[interferenceDimension] || {};
  const matrixEntries = [
    ["ET_0", result.matrices.et_0],
    ["ET*", result.matrices.et_star],
    ["Bandwidth BW", result.matrices.bandwidth_bw],
    ["Transfer delay", result.matrices.transfer_delay],
    ["Financial cost", result.matrices.financial_network_cost],
    ["Container overhead", result.matrices.container_overhead],
  ];
  return (
    <div className="matrix-stack">
      {matrixEntries.map(([title, matrix]) => <MatrixTable key={title} title={title} matrix={matrix} />)}
      <section className="data-section pairwise-toolbar">
        <div>
          <h2>Interference I_n</h2>
          <p>Interference is defined per machine and dimension.</p>
        </div>
        <div className="filter-row">
          <label className="control compact-control">
            <span>Machine</span>
            <select value={interferenceResourceId} onChange={(event) => setInterferenceResourceId(event.target.value)}>
              {result.resources.map((resource) => <option key={resource.id} value={resource.id}>{resource.name}</option>)}
            </select>
          </label>
          <label className="control compact-control">
            <span>Dimension</span>
            <select value={interferenceDimension} onChange={(event) => setInterferenceDimension(event.target.value)}>
              {interferenceDimensionOptions.map((dimension) => <option key={dimension} value={dimension}>{dimensionLabel(dimension)}</option>)}
            </select>
          </label>
        </div>
      </section>
      <MatrixTable
        title={`Interference I_n / ${interferenceResourceId} / ${dimensionLabel(interferenceDimension)}`}
        matrix={interferenceMatrix}
      />
    </div>
  );
}
