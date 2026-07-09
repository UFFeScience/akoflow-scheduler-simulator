export const weightKeys = ["weight_time", "weight_cost"];

export function normalizeWeights(currentWeights, changedKey, changedValue) {
  const editedUnits = clamp(Math.round(Number(changedValue) * 100), 0, 100);
  const remainingUnits = 100 - editedUnits;
  const otherKeys = weightKeys.filter((key) => key !== changedKey);
  const otherKey = otherKeys[0];

  return {
    [changedKey]: toWeight(editedUnits),
    [otherKey]: toWeight(remainingUnits),
  };
}

function toWeight(units) {
  return Number((units / 100).toFixed(2));
}

function clamp(value, min, max) {
  if (!Number.isFinite(value)) return min;
  return Math.min(max, Math.max(min, value));
}
