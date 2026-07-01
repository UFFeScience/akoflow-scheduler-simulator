import { useState } from 'react';
import { scalableTimeMatrices } from '../../lib/constants.js';
import { aggregateInterferenceDimension, buildAggregatedInterferenceMatrix } from '../../lib/interference.js';
import { dimensionLabel } from '../../lib/format.js';
import EditableMatricesToolbar from './EditableMatricesToolbar.jsx';
import EditableMatrixTable from './EditableMatrixTable.jsx';

export default function EditableMatricesView({ generated, onChange }) {
  const matrixEntries = [
    ["et_0", "ET_0"],
    ["et_star", "ET*"],
    ["bandwidth_bw", "Bandwidth BW"],
    ["transfer_delay", "Transfer delay"],
    ["financial_network_cost", "Financial cost"],
    ["container_overhead", "Container overhead"],
  ];
  const [activeMatrix, setActiveMatrix] = useState("et_0");
  const [interferenceResourceId, setInterferenceResourceId] = useState(generated.resources[0]?.id || "");
  const [interferenceDimension, setInterferenceDimension] = useState("cpu");
  const [timeMultiplier, setTimeMultiplier] = useState(1);
  const activeMatrixData = generated.matrices[activeMatrix] || {};
  const canScaleActiveMatrix = scalableTimeMatrices.has(activeMatrix);
  const isAggregateInterference = interferenceDimension === aggregateInterferenceDimension;
  const interferenceMatrix = isAggregateInterference
    ? buildAggregatedInterferenceMatrix(generated.matrices.interference_i_n, interferenceResourceId)
    : generated.matrices.interference_i_n[interferenceResourceId]?.[interferenceDimension] || {};

  function updateMatrixCell(matrixKey, rowKey, columnKey, value) {
    const numericValue = Number(value);
    if (!Number.isFinite(numericValue)) return;
    onChange((current) => {
      const next = structuredClone(current);
      next.matrices[matrixKey][rowKey][columnKey] = numericValue;
      if (matrixKey === "et_0") {
        next.matrices.et_star[rowKey][columnKey] = numericValue;
      }
      return next;
    });
  }

  function updateInterferenceCell(sourceTaskId, targetTaskId, value) {
    const numericValue = Number(value);
    if (!Number.isFinite(numericValue)) return;
    onChange((current) => {
      const next = structuredClone(current);
      next.matrices.interference_i_n[interferenceResourceId][interferenceDimension][sourceTaskId][targetTaskId] = numericValue;
      return next;
    });
  }

  function multiplyActiveMatrix() {
    const multiplier = Number(timeMultiplier);
    if (!Number.isFinite(multiplier) || multiplier < 0 || !canScaleActiveMatrix) return;
    onChange((current) => {
      const next = structuredClone(current);
      for (const rowKey of Object.keys(next.matrices[activeMatrix] || {})) {
        for (const columnKey of Object.keys(next.matrices[activeMatrix][rowKey] || {})) {
          next.matrices[activeMatrix][rowKey][columnKey] = Number((next.matrices[activeMatrix][rowKey][columnKey] * multiplier).toFixed(4));
        }
      }
      if (activeMatrix === "et_0") {
        next.matrices.et_star = structuredClone(next.matrices.et_0);
      }
      return next;
    });
  }

  return (
    <div className="steps-view"><section className="data-section step-header">
        <div>
          <span>Step 3</span>
          <h2>Edit generated matrices</h2>
          <p>Values are generated randomly first. Update any matrix values, then save and schedule from the left panel.</p>
        </div>
      </section>
      <EditableMatricesToolbar
        generated={generated}
        matrixEntries={matrixEntries}
        canScaleActiveMatrix={canScaleActiveMatrix}
        onScale={multiplyActiveMatrix}
        state={{ activeMatrix, interferenceResourceId, interferenceDimension, timeMultiplier }}
        setters={{ setActiveMatrix, setInterferenceResourceId, setInterferenceDimension, setTimeMultiplier }}
      />

      <EditableMatrixTable
        title={matrixEntries.find(([key]) => key === activeMatrix)?.[1] || activeMatrix}
        matrix={activeMatrixData}
        onChange={(row, column, value) => updateMatrixCell(activeMatrix, row, column, value)}
      />

      <EditableMatrixTable
        title={`Interference I_n / ${interferenceResourceId} / ${dimensionLabel(interferenceDimension)}`}
        matrix={interferenceMatrix}
        onChange={updateInterferenceCell}
        readOnly={isAggregateInterference}
      />
    </div>
  );
}
