export default function GanttDependencies({ dependencies, layouts, width, height }) {
  return (
    <svg className="gantt-dependencies" width={width} height={height}>
      <defs>
        <marker id="gantt-arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
          <path d="M0,0 L0,8 L8,4 z" />
        </marker>
      </defs>
      {dependencies.map((dependency) => {
        const source = layouts[dependency.source];
        const target = layouts[dependency.target];
        if (!source || !target) return null;
        const midX = target.x >= source.endX ? (source.endX + target.x) / 2 : source.endX + 24;
        return (
          <path
            key={`${dependency.source}-${dependency.target}`}
            d={`M ${source.endX} ${source.centerY} C ${midX} ${source.centerY}, ${midX} ${target.centerY}, ${target.x} ${target.centerY}`}
            markerEnd="url(#gantt-arrow)"
          />
        );
      })}
    </svg>
  );
}
