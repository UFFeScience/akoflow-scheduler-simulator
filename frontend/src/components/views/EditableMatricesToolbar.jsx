import { aggregateInterferenceDimension, interferenceDimensionOptions } from '../../lib/interference.js';
import { dimensionLabel } from '../../lib/format.js';

export default function EditableMatricesToolbar({ generated, state, setters, matrixEntries, canScaleActiveMatrix, onScale }) {
  return (
    <section className="data-section pairwise-toolbar">
      <div className="filter-row">
        <label className="control compact-control">
          <span>Matrix</span>
          <select value={state.activeMatrix} onChange={(event) => setters.setActiveMatrix(event.target.value)}>
            {matrixEntries.map(([key, label]) => <option key={key} value={key}>{label}</option>)}
          </select>
        </label>
        <label className="control compact-control">
          <span>Multiply time by X</span>
          <input type="number" min={0} step={0.1} value={state.timeMultiplier} onChange={(event) => setters.setTimeMultiplier(event.target.value)} disabled={!canScaleActiveMatrix} />
        </label>
        <button className="secondary-button matrix-scale-button" type="button" onClick={onScale} disabled={!canScaleActiveMatrix}>Apply x</button>
        <label className="control compact-control">
          <span>Interference machine</span>
          <select value={state.interferenceResourceId} onChange={(event) => setters.setInterferenceResourceId(event.target.value)}>
            {generated.resources.map((resource) => <option key={resource.id} value={resource.id}>{resource.name}</option>)}
          </select>
        </label>
        <label className="control compact-control">
          <span>Interference dimension</span>
          <select value={state.interferenceDimension} onChange={(event) => setters.setInterferenceDimension(event.target.value)}>
            {interferenceDimensionOptions.map((dimension) => <option key={dimension} value={dimension}>{dimensionLabel(dimension)}</option>)}
          </select>
        </label>
        {state.interferenceDimension === aggregateInterferenceDimension && <span className="status-message">Aggregate is read-only.</span>}
      </div>
    </section>
  );
}
