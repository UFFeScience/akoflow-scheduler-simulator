export const weightKeys = ["weight_time", "weight_cost", "weight_interference"];

export function normalizeWeights(currentWeights, changedKey, changedValue) {
  const editedUnits = clamp(Math.round(Number(changedValue) * 100), 0, 100);
  const remainingUnits = 100 - editedUnits;
  const otherKeys = weightKeys.filter((key) => key !== changedKey);
  const otherUnits = otherKeys.map((key) => clamp(Math.round(Number(currentWeights[key] || 0) * 100), 0, 100));
  const otherTotal = otherUnits[0] + otherUnits[1];

  let nextOtherUnits;
  if (otherTotal === 0) {
    const first = Math.floor(remainingUnits / 2);
    nextOtherUnits = [first, remainingUnits - first];
  } else {
    const first = Math.round((remainingUnits * otherUnits[0]) / otherTotal);
    nextOtherUnits = [first, remainingUnits - first];
  }

  return {
    [changedKey]: toWeight(editedUnits),
    [otherKeys[0]]: toWeight(nextOtherUnits[0]),
    [otherKeys[1]]: toWeight(nextOtherUnits[1]),
  };
}

function toWeight(units) {
  return Number((units / 100).toFixed(2));
}

function clamp(value, min, max) {
  if (!Number.isFinite(value)) return min;
  return Math.min(max, Math.max(min, value));
}
