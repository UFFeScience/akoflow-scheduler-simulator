export default function ControlInput({ label, value, min, max, step, onChange }) {
  return (
    <label className="control">
      <span>{label}</span>
      <input type="number" value={value} min={min} max={max} step={step} onChange={(event) => onChange(Number(event.target.value))} />
    </label>
  );
}
