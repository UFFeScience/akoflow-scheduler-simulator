export const interferenceDimensions = ["cpu", "memory", "io", "network"];
export const aggregateInterferenceDimension = "phi_n aggregated";
export const interferenceDimensionOptions = [...interferenceDimensions, aggregateInterferenceDimension];

export function buildAggregatedInterferenceMatrix(interferenceMatrixByResource, resourceId) {
  const resourceMatrix = interferenceMatrixByResource?.[resourceId] || {};
  const activityIds = Array.from(
    new Set(
      interferenceDimensions.flatMap((dimension) => {
        const dimensionMatrix = resourceMatrix[dimension] || {};
        return [
          ...Object.keys(dimensionMatrix),
          ...Object.values(dimensionMatrix).flatMap((targets) => Object.keys(targets || {})),
        ];
      }),
    ),
  );
  return Object.fromEntries(
    activityIds.map((sourceId) => [
      sourceId,
      Object.fromEntries(
        activityIds.map((targetId) => {
          const total = interferenceDimensions.reduce(
            (sum, dimension) => sum + (resourceMatrix[dimension]?.[sourceId]?.[targetId] || 0),
            0,
          );
          return [targetId, Number((total / interferenceDimensions.length).toFixed(4))];
        }),
      ),
    ]),
  );
}
