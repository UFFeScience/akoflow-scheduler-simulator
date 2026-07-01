export function buildResourceSpecs(clusterCount, cloudCount, coresPerMachine) {
  const resources = [];
  for (let index = 1; index <= clusterCount; index += 1) {
    resources.push(defaultResourceSpec("cluster", index, coresPerMachine));
  }
  for (let index = 1; index <= cloudCount; index += 1) {
    resources.push(defaultResourceSpec("cloud", index, coresPerMachine));
  }
  return resources;
}

export function syncResourceSpecs(currentSpecs, clusterCount, cloudCount, coresPerMachine) {
  const currentById = Object.fromEntries(currentSpecs.map((resource) => [resource.id, resource]));
  return buildResourceSpecs(clusterCount, cloudCount, coresPerMachine).map((resource) => ({
    ...resource,
    ...(currentById[resource.id] || {}),
  }));
}

export function defaultResourceSpec(kind, index, coresPerMachine) {
  const isCluster = kind === "cluster";
  return {
    id: `${isCluster ? "c" : "v"}${index}`,
    name: `${kind}-${index}`,
    kind,
    cores: coresPerMachine,
    memory: isCluster ? 16 + index * 4 : 24 + index * 8,
    bandwidth: isCluster ? 900 : 350,
    boot_overhead: isCluster ? 0 : 10 + index,
    location: isCluster ? "on-prem" : index % 2 === 0 ? "us-east" : "eu-west",
  };
}
