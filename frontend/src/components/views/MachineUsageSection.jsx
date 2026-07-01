export default function MachineUsageSection({ machineUse, assignmentCount }) {
  return (
    <section className="data-section">
      <h2>Machine usage</h2>
      <div className="machine-usage">
        {machineUse.map(({ resource, assignments }) => (
          <div className="usage-row" key={resource.id}>
            <span>{resource.name}</span>
            <div><i style={{ width: `${Math.max(4, (assignments.length / Math.max(assignmentCount, 1)) * 100)}%` }} /></div>
            <strong>{assignments.length}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}
