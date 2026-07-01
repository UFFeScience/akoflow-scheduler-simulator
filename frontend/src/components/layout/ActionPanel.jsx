import { ChevronLeft, Play } from 'lucide-react';

export default function ActionPanel({ phase, generated, status, statusMessage, onSave, onBackToWorkflow, onEditMatrices }) {
  return (
    <>
      <div className="brand-row">
        <div>
          <h1>Scheduler Simulator</h1>
          <p>{phase === "matrices" ? "Matrix review" : "Scheduled result"}</p>
        </div>
      </div>
      <section className="workflow-import">
        <div>
          <span>Current workflow</span>
          <strong>{generated?.workflow.preset || "-"}</strong>
        </div>
        <div className="mini-stats">
          <span>{generated?.workflow.tasks.length || 0} activities</span>
          <span>{generated?.workflow.dependencies.length || 0} dependencies</span>
          <span>{generated?.resources.length || 0} machines</span>
        </div>
      </section>
      {phase === "matrices" && (
        <>
          <button className="primary-button" onClick={onSave} disabled={status === "running" || !generated}>
            <Play size={17} />
            Save and schedule
          </button>
          <button className="secondary-button full-width-button" type="button" onClick={onBackToWorkflow}>
            Back to workflow
          </button>
        </>
      )}
      {phase === "results" && (
        <button className="primary-button" onClick={onEditMatrices} disabled={!generated}>
          <ChevronLeft size={17} />
          Edit matrices
        </button>
      )}
      {statusMessage && <p className="status-message error">{statusMessage}</p>}
    </>
  );
}
