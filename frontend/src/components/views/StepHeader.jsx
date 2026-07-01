import { ChevronLeft, ChevronRight } from 'lucide-react';

export default function StepHeader({ step, steps, stepIndex, task, onStepIndexChange }) {
  return (
    <section className="step-header data-section">
      <div>
        <span>Step {step.step} of {steps.length}</span>
        <h2>{task?.label || step.task_id}</h2>
        <p>{task?.workflow_stage} selected {step.selected_resource_id} / {step.selected_core_id}</p>
      </div>
      <div className="step-controls">
        <button className="secondary-button" onClick={() => onStepIndexChange((current) => Math.max(0, current - 1))} disabled={stepIndex === 0}>
          <ChevronLeft size={16} /> Previous
        </button>
        <button className="secondary-button" onClick={() => onStepIndexChange((current) => Math.min(steps.length - 1, current + 1))} disabled={stepIndex >= steps.length - 1}>
          Next <ChevronRight size={16} />
        </button>
      </div>
    </section>
  );
}
