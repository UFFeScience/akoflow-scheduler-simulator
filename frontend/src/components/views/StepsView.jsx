import { useState } from 'react';
import { fmt } from '../../lib/format.js';
import Metric from '../Metric.jsx';
import CandidateScoresTable from './CandidateScoresTable.jsx';
import SelectedPairwiseTable from './SelectedPairwiseTable.jsx';
import StepHeader from './StepHeader.jsx';

export default function StepsView({ result, onSelect }) {
  const [stepIndex, setStepIndex] = useState(0);
  const steps = result.scheduler_steps || [];
  const step = steps[Math.min(stepIndex, Math.max(steps.length - 1, 0))];
  const task = result.workflow.tasks.find((item) => item.id === step?.task_id);
  const selectedCandidate = step?.candidates.find((candidate) => candidate.selected);

  if (!step) {
    return <div className="empty-state">No scheduler steps were returned.</div>;
  }

  return (
    <div className="steps-view">
      <StepHeader step={step} steps={steps} stepIndex={stepIndex} task={task} onStepIndexChange={setStepIndex} />

      <div className="stats-grid">
        <Metric label="Candidates" value={step.candidates.length} />
        <Metric label="Selected score" value={fmt(step.selected_total_score)} />
        <Metric label="Selected ET*" value={fmt(selectedCandidate?.effective_runtime)} />
        <Metric label="Interference time" value={fmt(selectedCandidate?.interference_time)} />
      </div>

      {task?.run && (
        <section className="data-section">
          <h2>Activity command</h2>
          <pre>{task.run}</pre>
        </section>
      )}

      <CandidateScoresTable step={step} onSelect={onSelect} />
      <SelectedPairwiseTable selectedCandidate={selectedCandidate} />
    </div>
  );
}
