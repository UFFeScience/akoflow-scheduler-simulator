import { Download, Moon, RefreshCw, Sun } from 'lucide-react';
import { fmt } from '../../lib/format.js';
import Metric from '../Metric.jsx';

export default function Topbar({ phase, result, generated, request, workflowYaml, theme, onReset, onThemeToggle, onExport }) {
  const title = phase === "results" ? result?.workflow.preset : generated?.workflow.preset || request.preset;
  const subtitle = phase === "results" && result
    ? `${result.workflow.tasks.length} tasks / ${result.resources.length} machines${workflowYaml ? " / imported YAML" : ""}`
    : generated
      ? `${generated.workflow.tasks.length} tasks / ${generated.resources.length} machines / editing matrices`
      : "Select synthetic generation or import YAML";

  return (
    <header className="topbar">
      <div>
        <strong>{title}</strong>
        <span>{subtitle}</span>
      </div>
      <div className="metrics">
        <Metric label="Makespan" value={fmt(phase === "results" ? result?.scheduler_variables.makespan : null)} />
        <Metric label="Budget used" value={fmt(phase === "results" ? result?.scheduler_variables.b_used : null)} />
        <Metric label="Cost C_W" value={fmt(phase === "results" ? result?.cost_variables.c_w : null)} />
        <button className="icon-button" title="Reset flow" onClick={onReset}><RefreshCw size={18} /></button>
        <button className="icon-button" title="Toggle theme" onClick={onThemeToggle}>
          {theme === "light" ? <Moon size={18} /> : <Sun size={18} />}
        </button>
        <button className="icon-button" title="Export JSON" onClick={onExport} disabled={phase !== "results" || !result}>
          <Download size={18} />
        </button>
      </div>
    </header>
  );
}
