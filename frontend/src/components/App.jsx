import { useSimulatorController } from '../hooks/useSimulatorController.js';
import ActionPanel from './layout/ActionPanel.jsx';
import TabNav from './layout/TabNav.jsx';
import Topbar from './layout/Topbar.jsx';
import WorkspaceCanvas from './layout/WorkspaceCanvas.jsx';
import DetailsPanel from './views/DetailsPanel.jsx';

export default function App() {
  function exportJson() {
    if (!controller.result) return;
    const blob = new Blob([JSON.stringify(controller.result, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `${controller.result.id}.json`;
    anchor.click();
    URL.revokeObjectURL(url);
  }

  const controller = useSimulatorController();
  const shellClass = controller.phase === "workflow" ? "setup-shell" : controller.phase === "matrices" ? "matrix-shell" : "result-shell";

  return (
    <div className={`app-shell ${shellClass}`}>
      {controller.phase !== "workflow" && (
        <aside className="left-panel">
          <ActionPanel
            phase={controller.phase}
            generated={controller.generated}
            status={controller.status}
            statusMessage={controller.statusMessage}
            onSave={controller.saveMatricesAndSchedule}
            onBackToWorkflow={() => { controller.setPhase("workflow"); controller.setActiveTab("Workflow"); }}
            onEditMatrices={() => { controller.setPhase("matrices"); controller.setActiveTab("Matrices"); }}
          />
        </aside>
      )}
      <main className="workspace">
        <Topbar {...controller} onReset={controller.resetFlow} onThemeToggle={() => controller.setTheme((current) => (current === "light" ? "dark" : "light"))} onExport={exportJson} />
        <TabNav phase={controller.phase} activeTab={controller.activeTab} onChange={controller.setActiveTab} />
        <WorkspaceCanvas controller={controller} />
      </main>
      {controller.phase === "results" && (
        <aside className="right-panel">
          <DetailsPanel result={controller.result} assignment={controller.selectedAssignment} taskId={controller.selectedTaskId} />
        </aside>
      )}
    </div>
  );
}
