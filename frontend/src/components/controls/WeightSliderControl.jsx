export default function WeightSliderControl({ label, value, onChange, help }) {
  const percent = Math.round((Number(value) || 0) * 100);
  return (
    <label className="slider-control weight-control">
      <span>{label}</span>
      <div className="slider-row">
        <input type="range" value={value} min={0} max={1} step={0.01} onChange={(event) => onChange(Number(event.target.value))} />
        <strong>{percent}%</strong>
      </div>
      {help && <small>{help}</small>}
    </label>
  );
}
