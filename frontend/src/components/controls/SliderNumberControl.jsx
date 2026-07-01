export default function SliderNumberControl({ label, value, min, max, step, onChange, suffix = "", help }) {
  const numericValue = Number(value) || 0;
  const sliderMax = Math.max(max, numericValue);
  return (
    <label className="slider-control">
      <span>{label}</span>
      <div className="slider-row">
        <input
          type="range"
          value={numericValue}
          min={min}
          max={sliderMax}
          step={step}
          onChange={(event) => onChange(Number(event.target.value))}
        />
        <div className="number-with-suffix">
          <input type="number" value={numericValue} min={min} step={step} onChange={(event) => onChange(Number(event.target.value))} />
          {suffix && <em>{suffix}</em>}
        </div>
      </div>
      {help && <small>{help}</small>}
    </label>
  );
}
