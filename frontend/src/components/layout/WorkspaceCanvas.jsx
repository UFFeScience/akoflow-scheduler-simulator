import WorkflowStartScreen from '../views/WorkflowStartScreen.jsx';
import WorkflowPreviewView from '../views/WorkflowPreviewView.jsx';
import EditableMatricesView from '../views/EditableMatricesView.jsx';
import DagView from '../views/DagView.jsx';
import GanttView from '../views/GanttView.jsx';
import StepsView from '../views/StepsView.jsx';
import ActivityStatsView from '../views/ActivityStatsView.jsx';
import PairwiseInterferenceView from '../views/PairwiseInterferenceView.jsx';
import MachineView from '../views/MachineView.jsx';
import MatricesView from '../views/MatricesView.jsx';
import VariablesView from '../views/VariablesView.jsx';
import TablesView from '../views/TablesView.jsx';

export default function WorkspaceCanvas({ controller }) {
  const c = controller;
  return (
    <section className="canvas">
      {c.phase === "workflow" && <WorkflowStartScreen controller={c} />}
      {c.phase === "matrices" && c.generated && c.activeTab === "DAG" && <WorkflowPreviewView generated={c.generated} selectedTaskId={c.selectedTaskId} onSelect={c.setSelectedTaskId} />}
      {c.phase === "matrices" && c.generated && c.activeTab === "Matrices" && <EditableMatricesView generated={c.generated} onChange={c.setGenerated} />}
      {c.phase === "results" && c.result && c.activeTab === "DAG" && <DagView result={c.result} selectedTaskId={c.selectedTaskId} onSelect={c.setSelectedTaskId} />}
      {c.phase === "results" && c.result && c.activeTab === "Gantt" && <GanttView result={c.result} selectedTaskId={c.selectedTaskId} onSelect={c.setSelectedTaskId} />}
      {c.phase === "results" && c.result && c.activeTab === "Steps" && <StepsView result={c.result} onSelect={c.setSelectedTaskId} />}
      {c.phase === "results" && c.result && c.activeTab === "Stats" && <ActivityStatsView result={c.result} onSelect={c.setSelectedTaskId} />}
      {c.phase === "results" && c.result && c.activeTab === "Pairwise" && <PairwiseInterferenceView result={c.result} onSelect={c.setSelectedTaskId} />}
      {c.phase === "results" && c.result && c.activeTab === "Machines" && <MachineView result={c.result} selectedTaskId={c.selectedTaskId} onSelect={c.setSelectedTaskId} />}
      {c.phase === "results" && c.result && c.activeTab === "Matrices" && <MatricesView result={c.result} />}
      {c.phase === "results" && c.result && c.activeTab === "Variables" && <VariablesView result={c.result} />}
      {c.phase === "results" && c.result && c.activeTab === "Tables" && <TablesView result={c.result} onSelect={c.setSelectedTaskId} />}
    </section>
  );
}
