import { tabs } from '../../lib/constants.js';

export default function TabNav({ phase, activeTab, onChange }) {
  const visibleTabs = phase === "results" ? tabs : phase === "matrices" ? ["DAG", "Matrices"] : ["Workflow"];
  return (
    <nav className="tabs">
      {visibleTabs.map((tab) => (
        <button key={tab} className={activeTab === tab ? "active" : ""} onClick={() => onChange(tab)}>
          {tab}
        </button>
      ))}
    </nav>
  );
}
