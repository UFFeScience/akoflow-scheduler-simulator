import { useState } from 'react';
import { buildGanttContextByTask } from '../lib/ganttContext.js';
import { resourceColors } from '../lib/colors.js';

export function useGanttLayout(result) {
  const [visibleTiming, setVisibleTiming] = useState({ interference: false, container: false, boot: false, transfer: false, stopped: true });
  const [showDependencies, setShowDependencies] = useState(true);
  const stopIntervals = result.machine_stop_intervals || [];
  const maxVisibleFinish = result.assignments.reduce((maxValue, item) => {
    const baseRuntime = result.matrices.et_0[item.task_id]?.[item.resource_id] ?? item.effective_runtime;
    const interferenceRuntime = Math.max(0, item.effective_runtime - baseRuntime);
    return Math.max(maxValue, item.start_time + baseRuntime + (visibleTiming.interference ? interferenceRuntime : 0) + (visibleTiming.transfer ? item.transfer_delay : 0));
  }, result.scheduler_variables.makespan);
  const maxVisibleStopFinish = visibleTiming.stopped ? stopIntervals.reduce((maxValue, item) => Math.max(maxValue, item.boot_finish_time), maxVisibleFinish) : maxVisibleFinish;
  const minVisibleStart = result.assignments.reduce((minValue, item) => {
    const preRuntime = (visibleTiming.boot ? item.boot_overhead : 0) + (visibleTiming.container ? item.container_overhead : 0);
    return Math.min(minValue, item.start_time - preRuntime);
  }, 0);
  const minVisibleStopStart = visibleTiming.stopped ? stopIntervals.reduce((minValue, item) => Math.min(minValue, item.stop_time), minVisibleStart) : minVisibleStart;
  const timelineOrigin = Math.min(0, minVisibleStopStart);
  const timelineSpan = Math.max(1, maxVisibleStopFinish - timelineOrigin);
  const timelineWidth = Math.max(760, timelineSpan * 7);
  const scale = timelineWidth / timelineSpan;
  const laneTrackHeight = 38;
  const machineBorder = 1;
  const machineHeaderHeight = 49;
  const machineGap = 12;
  const coreGap = 8;
  const coresPaddingTop = 12;
  const coresPaddingBottom = 14;
  const trackOffsetX = machineBorder + 14 + 78 + 10 + machineBorder;
  const machinePositions = [];
  const lanePositions = {};
  let currentY = 0;

  for (const resource of result.resources) {
    machinePositions.push({ resource, y: currentY });
    for (const core of resource.cores) {
      lanePositions[core.id] = {
        y: currentY + machineBorder + machineHeaderHeight + coresPaddingTop + core.index * (laneTrackHeight + coreGap),
        resourceId: resource.id,
      };
    }
    currentY += machineBorder * 2 + machineHeaderHeight + coresPaddingTop + coresPaddingBottom
      + resource.cores.length * laneTrackHeight + Math.max(0, resource.cores.length - 1) * coreGap + machineGap;
  }

  function assignmentLayout(item) {
    const baseRuntime = result.matrices.et_0[item.task_id]?.[item.resource_id] ?? item.effective_runtime;
    const interferenceRuntime = Math.max(0, item.effective_runtime - baseRuntime);
    const preRuntime = (visibleTiming.boot ? item.boot_overhead : 0) + (visibleTiming.container ? item.container_overhead : 0);
    const executionRuntime = baseRuntime + (visibleTiming.interference ? interferenceRuntime : 0);
    const postRuntime = visibleTiming.transfer ? item.transfer_delay : 0;
    const x = (item.start_time - preRuntime - timelineOrigin) * scale;
    const width = Math.max(34, (preRuntime + executionRuntime + postRuntime) * scale);
    return { x, width, endX: x + width, centerY: (lanePositions[item.core_id]?.y || 0) + machineBorder + 5 + 13, y: lanePositions[item.core_id]?.y || 0 };
  }

  return {
    visibleTiming,
    showDependencies,
    stopIntervals,
    timelineOrigin,
    timelineWidth,
    scale,
    trackOffsetX,
    machinePositions,
    ganttBodyHeight: Math.max(1, currentY - machineGap),
    colorByResource: resourceColors(result.resources),
    ganttContextByTask: buildGanttContextByTask(result),
    assignmentLayoutByTask: Object.fromEntries(result.assignments.map((assignment) => [assignment.task_id, assignmentLayout(assignment)])),
    toggleTiming: (key) => setVisibleTiming((current) => ({ ...current, [key]: !current[key] })),
    toggleDependencies: () => setShowDependencies((current) => !current),
  };
}
