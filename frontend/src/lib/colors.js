export function resourceColors(resources) {
  const clusterPalette = ["#24a148", "#42be65", "#08bdba", "#0f62fe"];
  const cloudPalette = ["#f1c21b", "#ff832b", "#da1e28", "#a56eff"];
  let clusterIndex = 0;
  let cloudIndex = 0;
  return Object.fromEntries(
    resources.map((resource) => {
      const palette = resource.kind === "cluster" ? clusterPalette : cloudPalette;
      const index = resource.kind === "cluster" ? clusterIndex++ : cloudIndex++;
      return [resource.id, palette[index % palette.length]];
    }),
  );
}
