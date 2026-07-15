import { fmt } from '../../lib/format.js';

export default function ScheduleOptionsPanel({ response, selectedOptionId, onSelect }) {
  const options = response?.options || [];
  if (!options.length) return null;

  const machineSummary = (option) => Object.entries(option.machine_distribution || {})
    .sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))
    .slice(0, 4)
    .map(([resourceId, count]) => `${resourceId}:${count}`)
    .join(" ");
  const weightedSummary = (option) => {
    if (option.weighted_time_percent === undefined || option.weighted_cost_percent === undefined) {
      return fmt(option.weighted_score);
    }
    return `M ${fmt(option.weighted_time_percent)}% / C ${fmt(option.weighted_cost_percent)}%`;
  };

  return (
    <section className="schedule-options-panel">
      <header>
        <strong>Schedule options</strong>
        <span>{options.filter((option) => option.feasible).length} feasible / {options.length} returned</span>
      </header>
      <div className="schedule-options-table-wrap">
        <table className="schedule-options-table">
          <thead>
            <tr>
              {["rank", "status", "makespan", "budget", "violations", "weighted", "machines"].map((heading) => <th key={heading}>{heading}</th>)}
            </tr>
          </thead>
          <tbody>
            {options.map((option) => (
              <tr
                key={option.id}
                className={`${option.id === selectedOptionId ? "selected-row" : ""} ${option.feasible ? "feasible" : "infeasible"}`}
                onClick={() => onSelect(option.id)}
              >
                <td>#{option.rank}{option.recommended ? " rec." : ""}</td>
                <td>{option.feasible ? "feasible" : "infeasible"}</td>
                <td>{fmt(option.makespan)}</td>
                <td>{fmt(option.budget_used)}</td>
                <td>B {fmt(option.budget_violation)} / D {fmt(option.deadline_violation)}</td>
                <td title={`Score ${fmt(option.weighted_score)}`}>{weightedSummary(option)}</td>
                <td>{machineSummary(option)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
