#!/usr/bin/env python3
import csv, json, os
from pathlib import Path
import matplotlib.pyplot as plt
import pandas as pd
import seaborn as sns

root_name = os.environ.get("MATRIX_ROOT", "latency-datax-envs-workflows-complete-v1")
root = Path("/workspace/experiments/results") / root_name
records = []
for raw_path in root.glob("latency-*ms-bandwidth-*mbps/data-*x/sla-*/*/raw_results.csv"):
    parts = raw_path.relative_to(root).parts
    profile, data_dir, sla_dir, workflow = parts[:4]
    latency = int(profile.split("latency-")[1].split("ms")[0])
    bandwidth = float(profile.split("bandwidth-")[1].split("mbps")[0])
    data_scale = float(data_dir.removeprefix("data-").removesuffix("x"))
    manifest = json.loads((raw_path.parent / "manifest.json").read_text())
    rows = list(csv.DictReader(raw_path.open()))
    idx = {(r["scenario_id"], r["algorithm"]): r for r in rows}
    for scenario in sorted({r["scenario_id"] for r in rows}):
        heft, ptime, pcost = (idx[(scenario, a)] for a in ("heft_colocation", "prism_cc_time", "prism_cc_cost"))
        hm, tm, cm = map(lambda r: float(r["makespan"]), (heft, ptime, pcost))
        hc, tc, cc = map(lambda r: float(r["budget_used"]), (heft, ptime, pcost))
        records.append({
            "workflow": workflow, "seed": int(heft["interference_seed"]), "environment": scenario,
            "latency_ms": latency, "bandwidth_mbps": bandwidth, "data_scale": data_scale,
            "sla_label": sla_dir.removeprefix("sla-"), "budget_margin": manifest["budget_margin"],
            "deadline_margin": manifest["deadline_margin"], "deadline_limit": float(heft["deadline_limit"]),
            "cost_limit": float(heft["budget_limit"]), "heft_makespan_s": hm,
            "prism_time_makespan_s": tm, "prism_cost_makespan_s": cm, "heft_cost_usd": hc,
            "prism_time_cost_usd": tc, "prism_cost_cost_usd": cc,
            "time_gain_pct": 100*(hm-tm)/hm if hm else 0, "cost_saving_pct": 100*(hc-cc)/hc if hc else 0,
            "heft_feasible": heft["feasible"], "prism_time_feasible": ptime["feasible"],
            "prism_cost_feasible": pcost["feasible"], "result_directory": str(raw_path.parent.relative_to(Path('/workspace'))),
        })

df = pd.DataFrame(records).sort_values(["workflow","environment","latency_ms","data_scale","deadline_margin"])
root.mkdir(parents=True, exist_ok=True)
df.to_csv(root / "combined_results.csv", index=False)
(root / "combined_manifest.json").write_text(json.dumps({
    "matrix_root": root_name, "rows": len(df), "expected_rows": 1764,
    "network_profiles": [{"latency_ms":100,"bandwidth_mbps":500},{"latency_ms":1000,"bandwidth_mbps":100},{"latency_ms":10000,"bandwidth_mbps":10}],
    "data_scales": [1,10,100], "sla_margins": [1.2,0.95,0.9,0.8], "seed": 1,
    "aws_reference": "https://aws.amazon.com/ec2/pricing/on-demand/",
}, indent=2))

figdir = root / "combined-figures"; figdir.mkdir(exist_ok=True)
sns.set_theme(style="whitegrid")
def save(name):
    plt.tight_layout(); plt.savefig(figdir / name, dpi=180, bbox_inches="tight"); plt.close()

long_m = df.melt(id_vars=["workflow","latency_ms","data_scale","deadline_margin"], value_vars=["heft_makespan_s","prism_time_makespan_s","prism_cost_makespan_s"], var_name="algorithm", value_name="makespan")
sns.relplot(data=long_m, x="latency_ms", y="makespan", hue="algorithm", col="workflow", col_wrap=3, kind="line", marker="o", facet_kws={"sharey":False}, height=3)
save("01-makespan-versus-latencia-por-workflow.png")
long_c = df.melt(id_vars=["workflow","latency_ms","data_scale","deadline_margin"], value_vars=["heft_cost_usd","prism_time_cost_usd","prism_cost_cost_usd"], var_name="algorithm", value_name="cost")
sns.relplot(data=long_c, x="latency_ms", y="cost", hue="algorithm", col="workflow", col_wrap=3, kind="line", marker="o", facet_kws={"sharey":False}, height=3)
save("02-custo-versus-latencia-por-workflow.png")
for metric, name, title in [("time_gain_pct","03-ganho-time-latencia-data.png","Ganho PRISM-Time (%)"),("cost_saving_pct","04-economia-cost-latencia-data.png","Economia PRISM-Cost (%)")]:
    pivot=df.pivot_table(index=["workflow","data_scale"],columns="latency_ms",values=metric,aggfunc="mean")
    plt.figure(figsize=(9,10)); sns.heatmap(pivot,annot=True,fmt=".1f",cmap="RdYlGn"); plt.title(title); save(name)
feas=[]
for algorithm in ["heft","prism_time","prism_cost"]:
    temp=df.assign(algorithm=algorithm,feasible=df[f"{algorithm}_feasible"].astype(str).str.lower().eq("true").astype(float))
    feas.append(temp)
feasible=pd.concat(feas)
plt.figure(figsize=(10,5)); sns.lineplot(data=feasible,x="deadline_margin",y="feasible",hue="algorithm",style="data_scale",markers=True); plt.ylabel("Taxa de factibilidade"); save("05-factibilidade-por-sla-e-data.png")
plt.figure(figsize=(10,6)); sns.scatterplot(data=df,x="time_gain_pct",y="cost_saving_pct",hue="workflow",size="data_scale",style="latency_ms",alpha=.7); save("06-tradeoff-ganho-tempo-economia.png")
