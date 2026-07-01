import React from 'react';
import { fmt } from '../../lib/format.js';

export default function NodeLoadHeatmap({ timeBuckets, nodeTimeCells, onSelect }) {
  const maxRunning = Math.max(...nodeTimeCells.flatMap((item) => item.buckets.map((bucket) => bucket.count)), 1);
  return (
    <section className="data-section">
      <h2>Node load over time</h2>
      <div className="time-heatmap-wrap">
        <div className="time-heatmap" style={{ "--time-bucket-count": timeBuckets.length, "--time-row-count": Math.max(nodeTimeCells.length, 1) }}>
          <div className="time-heatmap-corner">Node</div>
          <div className="time-heatmap-axis">{timeBuckets.map((bucket) => <div key={bucket.index} title={`${fmt(bucket.start)}-${fmt(bucket.finish)}s`}>{bucket.start}</div>)}</div>
          {nodeTimeCells.map(({ resource, buckets }) => (
            <React.Fragment key={resource.id}>
              <div className="time-heatmap-node"><strong>{resource.name}</strong><span>{resource.kind === "cluster" ? "HPC" : "cloud"} / {resource.id}</span></div>
              <div className="time-heatmap-row">
                {buckets.map((bucket) => (
                  <button type="button" key={`${resource.id}-${bucket.index}`} className={`time-heatmap-cell ${resource.kind}`} style={{ "--heatmap-intensity": Math.max(0, bucket.count / maxRunning) }} title={`${resource.name}, ${fmt(bucket.start)}-${fmt(bucket.finish)}s: ${bucket.count} running`} onClick={() => bucket.running[0] && onSelect(bucket.running[0].task_id)}>
                    {bucket.count > 0 ? bucket.count : ""}
                  </button>
                ))}
              </div>
            </React.Fragment>
          ))}
        </div>
      </div>
    </section>
  );
}
