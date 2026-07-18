export const weightKeys = ["weight_time", "weight_cost"];

export const decisionDirections = {
  time: {
    key: "time",
    weightKey: "weight_time",
    label: "Finish earlier",
    help: "Prioritizes candidates with lower finish times.",
  },
  cost: {
    key: "cost",
    weightKey: "weight_cost",
    label: "Spend less",
    help: "Prioritizes candidates with lower CPU and memory execution cost.",
  },
};

export function getDecisionDirection(weights) {
  return Number(weights.weight_cost) > Number(weights.weight_time) ? "cost" : "time";
}

export function weightsForDecisionDirection(direction) {
  return direction === "cost"
    ? { weight_time: 0, weight_cost: 1 }
    : { weight_time: 1, weight_cost: 0 };
}
