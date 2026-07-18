import assert from "node:assert/strict";
import test from "node:test";

import { getDecisionDirection, weightsForDecisionDirection } from "./slaControls.js";

function total(weights) {
  return Number((weights.weight_time + weights.weight_cost).toFixed(2));
}

test("time direction enables only the time weight", () => {
  const next = weightsForDecisionDirection("time");

  assert.deepEqual(next, {
    weight_time: 1,
    weight_cost: 0,
  });
  assert.equal(total(next), 1);
});

test("cost direction enables only the cost weight", () => {
  const next = weightsForDecisionDirection("cost");

  assert.deepEqual(next, {
    weight_time: 0,
    weight_cost: 1,
  });
  assert.equal(total(next), 1);
});

test("existing proportional weights map to the dominant direction", () => {
  assert.equal(getDecisionDirection({ weight_time: 0.6, weight_cost: 0.4 }), "time");
  assert.equal(getDecisionDirection({ weight_time: 0.4, weight_cost: 0.6 }), "cost");
});

test("equal or missing weights default to time direction", () => {
  assert.equal(getDecisionDirection({ weight_time: 0.5, weight_cost: 0.5 }), "time");
  assert.equal(getDecisionDirection({}), "time");
});
