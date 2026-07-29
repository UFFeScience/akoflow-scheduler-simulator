from __future__ import annotations

import argparse
import json
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import seaborn as sns


LABELS = {
    "heft_classic": "HEFT clássico",
    "heft_colocation": "HEFT coalocado",
    "prism_cc_time": "PRISM-Time",
    "prism_cc_cost": "PRISM-Cost",
}
COLORS = {
    "heft_classic": "#6B7280",
    "heft_colocation": "#6B7280",
    "prism_cc_time": "#2563EB",
    "prism_cc_cost": "#16A34A",
}


def save(fig: plt.Figure, path: Path) -> None:
    fig.tight_layout()
    fig.savefig(path, dpi=180, bbox_inches="tight")
    plt.close(fig)


def ordered_algorithms(data: pd.DataFrame) -> list[str]:
    baseline = "heft_classic" if "heft_classic" in set(data.algorithm) else "heft_colocation"
    return [baseline, "prism_cc_time", "prism_cc_cost"]


def box(data: pd.DataFrame, metric: str, title: str, ylabel: str, path: Path) -> None:
    order = ordered_algorithms(data)
    fig, ax = plt.subplots(figsize=(10, 7))
    sns.boxplot(
        data=data, x="algorithm", y=metric, order=order,
        hue="algorithm", palette=COLORS, legend=False, ax=ax,
    )
    ax.set_xticklabels([LABELS[item] for item in order])
    ax.set_title(title)
    ax.set_xlabel("")
    ax.set_ylabel(ylabel)
    save(fig, path)


def seed_lines(data: pd.DataFrame, metric: str, title: str, ylabel: str, path: Path) -> None:
    fig, ax = plt.subplots(figsize=(12, 7))
    for algorithm in ordered_algorithms(data):
        subset = data[data.algorithm == algorithm].sort_values("interference_seed")
        ax.plot(subset.interference_seed, subset[metric], marker="o", markersize=3,
                label=LABELS[algorithm], color=COLORS[algorithm])
    ax.set_title(title)
    ax.set_xlabel("Semente de interferência")
    ax.set_ylabel(ylabel)
    ax.legend(frameon=False)
    save(fig, path)


def bar_summary(data: pd.DataFrame, metric: str, title: str, ylabel: str, path: Path) -> None:
    order = ordered_algorithms(data)
    summary = data.groupby("algorithm", as_index=False)[metric].mean()
    fig, ax = plt.subplots(figsize=(10, 7))
    sns.barplot(
        data=summary, x="algorithm", y=metric, order=order,
        hue="algorithm", palette=COLORS, legend=False, ax=ax,
    )
    ax.set_xticklabels([LABELS[item] for item in order])
    ax.set_title(title)
    ax.set_xlabel("")
    ax.set_ylabel(ylabel)
    save(fig, path)


def generate_edge_extras(data: pd.DataFrame, figures: Path, task_count: int) -> None:
    box(data, "interference_time", "Tempo total de interferência", "Tempo (s)", figures / "09-tempo-interferencia.png")
    box(data, "interference_pairs", "Pares interferentes efetivos", "Número de pares", figures / "10-pares-interferentes.png")

    violations = data.assign(
        deadline_violation_flag=data.deadline_violation.gt(0).astype(float),
        budget_violation_flag=data.budget_violation.gt(0).astype(float),
    ).melt(
        id_vars="algorithm",
        value_vars=["deadline_violation_flag", "budget_violation_flag"],
        var_name="limit", value_name="violation",
    )
    fig, ax = plt.subplots(figsize=(10, 7))
    sns.barplot(data=violations, x="algorithm", y="violation", hue="limit",
                order=ordered_algorithms(data), ax=ax)
    ax.set_xticklabels([LABELS[item] for item in ordered_algorithms(data)])
    ax.set_title("Frequência de violações do SLA")
    ax.set_xlabel("")
    ax.set_ylabel("Proporção de execuções")
    save(fig, figures / "11-violacoes-sla.png")

    seed_lines(data, "makespan", "Makespan por semente", "Makespan (s)", figures / "12-makespan-por-semente.png")
    seed_lines(data, "budget_used", "Custo por semente", "Custo (USD)", figures / "13-custo-por-semente.png")

    for metric, title, ylabel, filename in [
        ("makespan", "Distribuição acumulada do makespan", "Probabilidade acumulada", "14-ecdf-makespan.png"),
        ("budget_used", "Distribuição acumulada do custo", "Probabilidade acumulada", "15-ecdf-custo.png"),
    ]:
        fig, ax = plt.subplots(figsize=(10, 7))
        sns.ecdfplot(data=data, x=metric, hue="algorithm", hue_order=ordered_algorithms(data),
                     palette=COLORS, ax=ax)
        ax.set_title(title)
        ax.set_xlabel("Makespan (s)" if metric == "makespan" else "Custo (USD)")
        ax.set_ylabel(ylabel)
        save(fig, figures / filename)

    means = data.groupby("algorithm")[["makespan", "budget_used", "algorithm_milliseconds", "interference_time"]].mean()
    normalized = means.div(means.max(axis=0)).reindex(ordered_algorithms(data))
    fig, ax = plt.subplots(figsize=(10, 6))
    sns.heatmap(normalized, annot=True, fmt=".2f", cmap="YlGnBu", ax=ax)
    ax.set_yticklabels([LABELS[item] for item in normalized.index], rotation=0)
    ax.set_title("Métricas médias normalizadas")
    save(fig, figures / "16-metricas-normalizadas.png")

    risk = data.groupby("algorithm").makespan.agg(["mean", "std"])
    risk["cv"] = risk["std"] / risk["mean"].replace(0, np.nan)
    bar_summary(risk.reset_index(), "cv", "Risco: coeficiente de variação do makespan", "CV", figures / "17-risco-makespan.png")

    baseline = ordered_algorithms(data)[0]
    paired = data.pivot(index="interference_seed", columns="algorithm", values=["makespan", "budget_used"])
    gains = []
    for algorithm in ["prism_cc_time", "prism_cc_cost"]:
        for metric, label in [("makespan", "Makespan"), ("budget_used", "Custo")]:
            value = 100 * (paired[metric][baseline] - paired[metric][algorithm]) / paired[metric][baseline].clip(lower=1e-9)
            gains.extend({"algorithm": LABELS[algorithm], "metric": label, "gain": item} for item in value)
    gains = pd.DataFrame(gains)
    for metric, filename in [("Makespan", "18-ganho-pareado-makespan.png"), ("Custo", "19-ganho-pareado-custo.png")]:
        fig, ax = plt.subplots(figsize=(10, 7))
        sns.boxplot(data=gains[gains.metric == metric], x="algorithm", y="gain", ax=ax)
        ax.axhline(0, color="#6B7280", linestyle="--")
        ax.set_title(f"Ganho pareado de {metric.lower()} sobre o baseline")
        ax.set_xlabel("")
        ax.set_ylabel("Ganho (%)")
        save(fig, figures / filename)

    utilization, allocation = [], []
    for _, row in data.iterrows():
        for machine, value in json.loads(row.machine_utilization).items():
            utilization.append({"algorithm": row.algorithm, "machine": machine, "value": value})
        for machine, value in json.loads(row.machine_distribution).items():
            allocation.append({"algorithm": row.algorithm, "machine": machine, "value": value / task_count})
    for frame, title, filename, fmt in [
        (pd.DataFrame(utilization), "Utilização média por máquina", "20-heatmap-utilizacao.png", ".3f"),
        (pd.DataFrame(allocation), "Fração média de tarefas por máquina", "21-heatmap-alocacao.png", ".2f"),
    ]:
        matrix = frame.groupby(["algorithm", "machine"]).value.mean().unstack().reindex(ordered_algorithms(data))
        fig, ax = plt.subplots(figsize=(12, 5))
        sns.heatmap(matrix, annot=True, fmt=fmt, cmap="Blues", ax=ax)
        ax.set_yticklabels([LABELS[item] for item in matrix.index], rotation=0)
        ax.set_title(title)
        save(fig, figures / filename)

    derived = data.assign(
        cost_per_task=data.budget_used / task_count,
        makespan_per_task=data.makespan / task_count,
        budget_headroom=100 * (data.budget_limit - data.budget_used) / data.budget_limit.clip(lower=1e-9),
        deadline_headroom=100 * (data.deadline_limit - data.makespan) / data.deadline_limit.clip(lower=1e-9),
    )
    for metric, title, ylabel, filename in [
        ("cost_per_task", "Custo médio por atividade", "USD por atividade", "22-custo-por-atividade.png"),
        ("makespan_per_task", "Makespan normalizado por atividade", "Segundos por atividade", "23-makespan-por-atividade.png"),
        ("budget_headroom", "Folga relativa do budget", "Folga (%)", "24-folga-budget.png"),
        ("deadline_headroom", "Folga relativa do deadline", "Folga (%)", "25-folga-deadline.png"),
    ]:
        box(derived, metric, title, ylabel, figures / filename)


def plot_26(data: pd.DataFrame, figures: Path) -> None:
    order = ordered_algorithms(data)
    means = data.groupby("algorithm")[["makespan", "budget_used", "algorithm_milliseconds"]].mean().reindex(order)
    baseline = means.loc[order[0]]
    relative = means.div(baseline).rename(columns={
        "makespan": "Makespan relativo",
        "budget_used": "Custo relativo",
        "algorithm_milliseconds": "Tempo do algoritmo relativo",
    })
    relative.index = [LABELS[item] for item in order]
    fig, ax = plt.subplots(figsize=(11, 7))
    relative.T.plot(
        kind="bar", ax=ax, color=[COLORS[item] for item in order],
    )
    ax.axhline(
        1, color="#111827", linestyle="--", linewidth=1,
        label="Referência do baseline",
    )
    ax.set_title("Resumo executivo relativo ao HEFT")
    ax.set_xlabel("")
    ax.set_ylabel("Razão em relação ao baseline")
    ax.set_xticklabels(relative.columns, rotation=0)
    handles, labels = ax.get_legend_handles_labels()
    ax.legend(handles, labels, frameon=False)
    save(fig, figures / "26-resumo-executivo.png")


def ensure_readme(figures: Path) -> None:
    path = figures / "README.md"
    text = path.read_text(encoding="utf-8") if path.exists() else "# Gráficos do protocolo experimental\n"
    additions = []
    for image in sorted(figures.glob("*.png")):
        if image.name in text:
            continue
        title = image.stem.split("-", 1)[-1].replace("-", " ").capitalize()
        additions.extend(["", f"## {title}", "", f"![{title}]({image.name})"])
    if additions:
        path.write_text(text.rstrip() + "\n" + "\n".join(additions) + "\n", encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("result_dir", type=Path)
    args = parser.parse_args()
    data = pd.read_csv(args.result_dir / "raw_results.csv")
    manifest = json.loads((args.result_dir / "manifest.json").read_text())
    figures = args.result_dir / "figures"
    figures.mkdir(exist_ok=True)
    sns.set_theme(style="whitegrid", context="talk")
    if len(list(figures.glob("*.png"))) < 25:
        generate_edge_extras(data, figures, manifest["task_count"])
    plot_26(data, figures)
    ensure_readme(figures)


if __name__ == "__main__":
    main()
