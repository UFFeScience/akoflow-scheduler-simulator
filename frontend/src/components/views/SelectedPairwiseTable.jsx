import { fmt } from '../../lib/format.js';

export default function SelectedPairwiseTable({ selectedCandidate }) {
  const pairs = selectedCandidate?.pairwise_interference.length
    ? selectedCandidate.pairwise_interference
    : [{ other_task_id: "none", value: 0, dimensions: {} }];
  return (
    <section className="data-section">
      <h2>Pairwise interference for selected candidate</h2>
      <div className="table-scroll">
        <table>
          <thead><tr>{["other activity", "pair value", "core", "memory", "io", "network"].map((heading) => <th key={heading}>{heading}</th>)}</tr></thead>
          <tbody>
            {pairs.map((pair) => (
              <tr key={pair.other_task_id}>
                <td>{pair.other_task_id}</td><td>{fmt(pair.value)}</td><td>{fmt(pair.dimensions.cpu)}</td><td>{fmt(pair.dimensions.memory)}</td><td>{fmt(pair.dimensions.io)}</td><td>{fmt(pair.dimensions.network)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
