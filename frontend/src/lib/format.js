export function fmt(value) {
  if (value === undefined || value === null) return "-";
  if (typeof value !== "number") return value;
  return Number.isInteger(value) ? String(value) : value.toFixed(3).replace(/0+$/, "").replace(/\.$/, "");
}

export function dimensionLabel(value) {
  return value === "cpu" ? "core" : value;
}

export function compactLabel(value, maxLength) {
  if (!value || value.length <= maxLength) return value;
  return `${value.slice(0, maxLength - 1)}...`;
}
