import assert from "node:assert/strict";
import test from "node:test";

import { normalizeWeights } from "./slaControls.js";

function total(weights) {
  return Number((weights.weight_time + weights.weight_cost + weights.weight_interference).toFixed(2));
}

test("increasing one weight reduces the other weights proportionally", () => {
  const next = normalizeWeights(
    { weight_time: 0.55, weight_cost: 0.3, weight_interference: 0.15 },
    "weight_time",
    0.7,
  );

  assert.deepEqual(next, {
    weight_time: 0.7,
    weight_cost: 0.2,
    weight_interference: 0.1,
  });
  assert.equal(total(next), 1);
});

test("setting one weight to all priority clears the other weights", () => {
  const next = normalizeWeights(
    { weight_time: 0.55, weight_cost: 0.3, weight_interference: 0.15 },
    "weight_cost",
    1,
  );

  assert.deepEqual(next, {
    weight_cost: 1,
    weight_time: 0,
    weight_interference: 0,
  });
  assert.equal(total(next), 1);
});

test("zero-valued peer weights split the remainder evenly", () => {
  const next = normalizeWeights(
    { weight_time: 1, weight_cost: 0, weight_interference: 0 },
    "weight_time",
    0.4,
  );

  assert.deepEqual(next, {
    weight_time: 0.4,
    weight_cost: 0.3,
    weight_interference: 0.3,
  });
  assert.equal(total(next), 1);
});

test("rounded values always sum to one", () => {
  const next = normalizeWeights(
    { weight_time: 0.34, weight_cost: 0.33, weight_interference: 0.33 },
    "weight_interference",
    0.34,
  );

  assert.equal(total(next), 1);
});
