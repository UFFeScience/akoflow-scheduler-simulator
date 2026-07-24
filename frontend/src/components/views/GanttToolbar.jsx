export default function GanttToolbar({ visibleTiming, showDependencies, laneMode, onToggleTiming, onToggleDependencies, onLaneModeChange }) {
  const checks = [
    ["interference", "Interference overhead"],
    ["container", "Container overhead"],
    ["boot", "Boot overhead"],
    ["stopped", "Stopped machines"],
    ["transfer", "Transfer delay after execution"],
  ];
  return (
    <div className="gantt-toolbar">
      <div className="gantt-toolbar-main">
        <div className="segmented-control" aria-label="Gantt lane mode">
          <button type="button" className={laneMode === "compact" ? "active" : ""} onClick={() => onLaneModeChange("compact")}>Compact</button>
          <button type="button" className={laneMode === "all" ? "active" : ""} onClick={() => onLaneModeChange("all")}>All cores</button>
        </div>
        <div className="gantt-checks">
          {checks.map(([key, label]) => (
            <label className="checkbox-control" key={key}>
              <input type="checkbox" checked={visibleTiming[key]} onChange={() => onToggleTiming(key)} />
              <span>{label}</span>
            </label>
          ))}
          <label className="checkbox-control">
            <input type="checkbox" checked={showDependencies} onChange={onToggleDependencies} />
            <span>Dependency lines</span>
          </label>
        </div>
      </div>
      <div className="gantt-legend">
        <span><i className="legend-swatch base" />Execution (solid)</span>
        <span><i className="legend-swatch interference" />Interference overhead (checker)</span>
        <span><i className="legend-swatch container" />Container overhead before execution (checker)</span>
        <span><i className="legend-swatch boot" />Boot overhead before execution (checker)</span>
        <span><i className="legend-swatch stopped" />Machine stopped</span>
        <span><i className="legend-swatch transfer" />Transfer delay after execution (checker)</span>
      </div>
    </div>
  );
}
