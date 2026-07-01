import { fmt } from '../../lib/format.js';

export default function CandidateScoresTable({ step, onSelect }) {
  const headings = ["rank", "machine", "core", "selected", "ST", "FT", "ET_0", "ET*", "interference", "phi", "cost", "time score", "cost score", "total"];
  return (
    <section className="data-section">
      <h2>Candidate machine scores</h2>
      <div className="table-scroll">
        <table>
          <thead><tr>{headings.map((heading) => <th key={heading}>{heading}</th>)}</tr></thead>
          <tbody>
            {step.candidates.map((candidate) => (
              <tr key={`${candidate.resource_id}-${candidate.core_id}`} className={candidate.selected ? "selected-row" : ""} onClick={() => onSelect(candidate.task_id)}>
                <td>{candidate.rank}</td><td>{candidate.resource_id}</td><td>{candidate.core_id}</td><td>{candidate.selected ? "yes" : "no"}</td><td>{fmt(candidate.start_time)}</td><td>{fmt(candidate.finish_time)}</td><td>{fmt(candidate.base_runtime)}</td><td>{fmt(candidate.effective_runtime)}</td><td>{fmt(candidate.interference_time)}</td><td>{fmt(candidate.phi_n)}</td><td>{fmt(candidate.raw_cost)}</td><td>{fmt(candidate.score.time_score)}</td><td>{fmt(candidate.score.cost_score)}</td><td>{fmt(candidate.score.total_score)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
