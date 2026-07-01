import { fmt } from '../../lib/format.js';

export default function MatrixTable({ title, matrix }) {
  const rows = Object.keys(matrix || {});
  const columns = Array.from(new Set(rows.flatMap((row) => Object.keys(matrix[row] || {}))));
  return (
    <section className="data-section">
      <h2>{title}</h2>
      <div className="table-scroll">
        <table>
          <thead>
            <tr><th></th>{columns.map((column) => <th key={column}>{column}</th>)}</tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row}>
                <th>{row}</th>
                {columns.map((column) => <td key={column}>{fmt(matrix[row]?.[column])}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
