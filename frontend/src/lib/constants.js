import { buildResourceSpecs } from './resourceSpecs.js';

export const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8000';
export const tabs = ["DAG", "Gantt", "Steps", "Stats", "Pairwise", "Machines", "Matrices", "Variables", "Tables"];
export const scalableTimeMatrices = new Set(["et_0", "et_star", "transfer_delay", "container_overhead"]);

export const defaultRequest = {
  preset: "Montage",
  seed: 42,
  task_count: 12,
  edge_density: 0.22,
  cluster_machines: 3,
  cloud_machines: 2,
  cores_per_machine: 2,
  resource_specs: buildResourceSpecs(3, 2, 2),
  weight_time: 0.6,
  weight_cost: 0.4,
  budget_limit: null,
  deadline_limit: null,
  option_count: 5,
};
