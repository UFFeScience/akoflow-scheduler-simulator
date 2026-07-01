import { fmt } from '../../lib/format.js';

export default function EditableMatrixTable({ title, matrix, onChange, readOnly = false }) {
  const rows = Object.keys(matrix || {});
  const columns = Array.from(new Set(rows.flatMap((row) => Object.keys(matrix[row] || {}))));
  return (
    <section className="data-section">
      <h2>{title}</h2>
      <div className="table-scroll editable-table-scroll">
        <table>
          <thead>
            <tr><th></th>{columns.map((column) => <th key={column}>{column}</th>)}</tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row}>
                <th>{row}</th>
                {columns.map((column) => (
                  <td key={column}>
                    {readOnly ? (
                      <span className="matrix-readonly">{fmt(matrix[row]?.[column] ?? 0)}</span>
                    ) : (
                      <input
                        className="matrix-input"
                        type="number"
                        step="0.0001"
                        value={matrix[row]?.[column] ?? 0}
                        onChange={(event) => onChange(row, column, event.target.value)}
                      />
                    )}
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
