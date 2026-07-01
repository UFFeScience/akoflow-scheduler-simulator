export default function VariablesView({ result }) {
  const groups = [
    ["Timing", result.timing_variables],
    ["Scheduler", result.scheduler_variables],
    ["Cost", result.cost_variables],
    ["Interference", result.interference_variables],
    ["Deviation", result.deviation_variables],
  ];
  return (
    <div className="variables">
      {groups.map(([title, values]) => (
        <section className="data-section" key={title}>
          <h2>{title}</h2>
          <pre>{JSON.stringify(values, null, 2)}</pre>
        </section>
      ))}
    </div>
  );
}
