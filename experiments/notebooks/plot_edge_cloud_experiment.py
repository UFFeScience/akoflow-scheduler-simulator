from __future__ import annotations

import argparse
import json
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd
import seaborn as sns


LABELS = {
    "heft_classic": "HEFT clássico",
    "heft_colocation": "HEFT coalocado",
    "prism_cc_time": "PRISM-Time",
    "prism_cc_cost": "PRISM-Cost",
}
ORDER = list(LABELS)
COLORS = ["#6B7280", "#2563EB", "#16A34A"]


def save(fig: plt.Figure, path: Path) -> None:
    fig.tight_layout()
    fig.savefig(path, dpi=180, bbox_inches="tight")
    plt.close(fig)


def main() -> None:
    global ORDER
    parser = argparse.ArgumentParser()
    parser.add_argument("result_dir", type=Path)
    args = parser.parse_args()
    data = pd.read_csv(args.result_dir / "raw_results.csv")
    baseline_algorithm = "heft_colocation" if "heft_colocation" in set(data.algorithm) else "heft_classic"
    ORDER = [baseline_algorithm, "prism_cc_time", "prism_cc_cost"]
    data["algorithm_label"] = data.algorithm.map(LABELS)
    figures = args.result_dir / "figures"
    figures.mkdir(exist_ok=True)
    sns.set_theme(style="whitegrid", context="talk")

    for metric, title, ylabel, filename, sla_column in [
        ("makespan", "Makespan no cenário edge–cloud extremo", "Tempo (s)", "01-makespan.png", "deadline_limit"),
        ("budget_used", "Custo no cenário edge–cloud extremo", "Custo (USD)", "02-custo.png", "budget_limit"),
    ]:
        fig, ax = plt.subplots(figsize=(10, 7))
        sns.boxplot(data=data, x="algorithm_label", y=metric, order=[LABELS[x] for x in ORDER], palette=COLORS, ax=ax)
        ax.axhline(data[sla_column].iloc[0], color="#DC2626", linestyle="--", label="Limite do SLA")
        ax.set_title(title)
        ax.set_xlabel("")
        ax.set_ylabel(ylabel)
        ax.legend(frameon=False)
        save(fig, figures / filename)

    summary = data.groupby("algorithm", as_index=False).agg(
        makespan=("makespan", "mean"),
        budget_used=("budget_used", "mean"),
        feasible=("feasible", "mean"),
        algorithm_milliseconds=("algorithm_milliseconds", "mean"),
    )
    summary["algorithm_label"] = summary.algorithm.map(LABELS)
    for metric, title, ylabel, filename in [
        ("feasible", "Factibilidade simultânea do SLA", "Execuções factíveis", "03-factibilidade.png"),
        ("algorithm_milliseconds", "Tempo do algoritmo", "Tempo (ms)", "04-tempo-algoritmo.png"),
    ]:
        fig, ax = plt.subplots(figsize=(10, 7))
        sns.barplot(data=summary, x="algorithm_label", y=metric, order=[LABELS[x] for x in ORDER], palette=COLORS, ax=ax)
        if metric == "feasible":
            ax.set_ylim(0, 1.05)
            for bar in ax.patches:
                ax.text(bar.get_x() + bar.get_width() / 2, bar.get_height(), f"{100*bar.get_height():.0f}%", ha="center", va="bottom")
        ax.set_title(title)
        ax.set_xlabel("")
        ax.set_ylabel(ylabel)
        save(fig, figures / filename)

    fig, ax = plt.subplots(figsize=(10, 7))
    for algorithm, color in zip(ORDER, COLORS):
        subset = data[data.algorithm == algorithm]
        ax.scatter(subset.makespan, subset.budget_used, s=55, alpha=.75, label=LABELS[algorithm], color=color)
    ax.set_title("Compromisso entre makespan e custo")
    ax.set_xlabel("Makespan (s)")
    ax.set_ylabel("Custo (USD)")
    ax.legend(frameon=False)
    save(fig, figures / "05-custo-vs-makespan.png")

    distributions = []
    utilizations = []
    for _, row in data.iterrows():
        for machine, count in json.loads(row.machine_distribution).items():
            distributions.append({"algorithm": LABELS[row.algorithm], "machine": machine, "tasks": count})
        for machine, value in json.loads(row.machine_utilization).items():
            utilizations.append({"algorithm": LABELS[row.algorithm], "machine": machine, "utilization": value})
    for frame, metric, title, ylabel, filename in [
        (pd.DataFrame(distributions), "tasks", "Distribuição média das tarefas", "Número de tarefas", "06-distribuicao-maquinas.png"),
        (pd.DataFrame(utilizations), "utilization", "Utilização média das máquinas", "Utilização", "07-utilizacao-maquinas.png"),
    ]:
        fig, ax = plt.subplots(figsize=(13, 7))
        sns.barplot(data=frame, x="machine", y=metric, hue="algorithm", hue_order=[LABELS[x] for x in ORDER], ax=ax)
        ax.set_title(title)
        ax.set_xlabel("")
        ax.set_ylabel(ylabel)
        ax.tick_params(axis="x", rotation=15)
        ax.legend(frameon=False)
        save(fig, figures / filename)

    baseline = summary.set_index("algorithm").loc[baseline_algorithm]
    gains = []
    for algorithm in ["prism_cc_time", "prism_cc_cost"]:
        row = summary.set_index("algorithm").loc[algorithm]
        gains.extend([
            {"algorithm": LABELS[algorithm], "metric": "Makespan", "gain": 100 * (baseline.makespan - row.makespan) / baseline.makespan},
            {"algorithm": LABELS[algorithm], "metric": "Custo", "gain": 100 * (baseline.budget_used - row.budget_used) / baseline.budget_used},
        ])
    fig, ax = plt.subplots(figsize=(10, 7))
    sns.barplot(data=pd.DataFrame(gains), x="algorithm", y="gain", hue="metric", ax=ax)
    ax.axhline(0, color="#6B7280", linewidth=1)
    ax.set_title("Ganho médio em relação ao HEFT clássico")
    ax.set_xlabel("")
    ax.set_ylabel("Ganho (%)")
    ax.legend(frameon=False)
    save(fig, figures / "08-ganhos-vs-heft.png")

    links = "\n".join(f"- ![{path.stem}](figures/{path.name})" for path in sorted(figures.glob("*.png")))
    (args.result_dir / "README.md").write_text(
        f"# PRISM × {LABELS[baseline_algorithm]} — edge–cloud extremo\n\n"
        "Montage 6.448, 30 sementes e SLA definido pelo HEFT clássico com margem de 1,2×.\n\n"
        f"{links}\n",
        encoding="utf-8",
    )


if __name__ == "__main__":
    main()
