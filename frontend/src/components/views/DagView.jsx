import { resourceColors } from '../../lib/colors.js';
import { compactLabel } from '../../lib/format.js';

export default function DagView({ result, selectedTaskId, onSelect }) {
  const assignmentByTask = Object.fromEntries((result.assignments || []).map((item) => [item.task_id, item]));
  const colorByResource = resourceColors(result.resources);
  const positions = result.workflow.tasks.map((task, index) => {
    const columns = Math.ceil(Math.sqrt(result.workflow.tasks.length * 1.8));
    const col = index % columns;
    const row = Math.floor(index / columns);
    return { task, x: 110 + col * 210, y: 70 + row * 115 };
  });
  const positionByTask = Object.fromEntries(positions.map((item) => [item.task.id, item]));
  const width = Math.max(900, Math.max(...positions.map((item) => item.x)) + 120);
  const height = Math.max(520, Math.max(...positions.map((item) => item.y)) + 110);

  return (
    <svg className="dag" viewBox={`0 0 ${width} ${height}`}>
      <defs>
        <marker id="arrow" markerWidth="10" markerHeight="10" refX="7" refY="3" orient="auto">
          <path d="M0,0 L0,6 L8,3 z" fill="#667085" />
        </marker>
      </defs>
      {result.workflow.dependencies.map((edge) => {
        const source = positionByTask[edge.source];
        const target = positionByTask[edge.target];
        return <line key={`${edge.source}-${edge.target}`} x1={source.x + 78} y1={source.y + 20} x2={target.x - 82} y2={target.y + 20} markerEnd="url(#arrow)" />;
      })}
      {positions.map(({ task, x, y }) => {
        const resourceId = assignmentByTask[task.id]?.resource_id;
        return (
          <g key={task.id} className={selectedTaskId === task.id ? "selected-node" : ""} onClick={() => onSelect(task.id)}>
            <rect x={x - 84} y={y - 18} width="168" height="52" rx="4" fill={colorByResource[resourceId] || "#0f62fe"} />
            <text x={x} y={y + 1} textAnchor="middle">{compactLabel(task.id, 22)}</text>
            <text x={x} y={y + 19} textAnchor="middle">{task.workflow_stage}</text>
          </g>
        );
      })}
    </svg>
  );
}
