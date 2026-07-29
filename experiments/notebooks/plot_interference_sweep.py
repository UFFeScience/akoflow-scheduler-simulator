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
    "heft_colocation": "HEFT com co-location",
    "prism_cc_time": "PRISM-Time",
    "prism_cc_cost": "PRISM-Cost",
}
COLORS = {
    "heft_colocation": "#6B7280",
    "prism_cc_time": "#2563EB",
    "prism_cc_cost": "#16A34A",
}
ORDER = ["heft_colocation", "prism_cc_time", "prism_cc_cost"]


def load_sweep(result_dir: Path) -> pd.DataFrame:
    frames = []
    for rate_dir in sorted(result_dir.glob("rate-*"), key=lambda path: int(path.name.split("-")[1])):
        frame = pd.read_csv(rate_dir / "raw_results.csv")
        frames.append(frame)
    if not frames:
        raise RuntimeError(f"nenhum resultado rate-* encontrado em {result_dir}")
    data = pd.concat(frames, ignore_index=True)
    data["interference_percent"] = data["interference_rate"] * 100
    data["algorithm_label"] = data["algorithm"].map(LABELS)
    return data.sort_values(["interference_percent", "algorithm"])


def priority_label(result_dir: Path) -> str:
    with (result_dir / "rate-10" / "manifest.json").open() as handle:
        policy = json.load(handle)["prism_cc_priority"]
    return {
        "upward_rank": "Upward Rank fixo",
        "ready_lookahead": "tarefas prontas + lookahead",
    }.get(policy, policy)


def plot_sweep(data: pd.DataFrame, output: Path, policy: str) -> None:
    sns.set_theme(style="whitegrid", context="talk")
    fig, axes = plt.subplots(2, 2, figsize=(17, 12))
    specs = [
        ("makespan", "Makespan", "Tempo (s)"),
        ("budget_used", "Custo total", "Custo (USD)"),
        ("interference_time", "Interferência acumulada", "Tempo de interferência (s)"),
    ]
    for ax, (metric, title, ylabel) in zip(axes.flat[:3], specs):
        for algorithm in ORDER:
            subset = data[data.algorithm == algorithm]
            ax.plot(
                subset.interference_percent,
                subset[metric],
                marker="o",
                linewidth=2.4,
                markersize=7,
                label=LABELS[algorithm],
                color=COLORS[algorithm],
            )
        ax.set_title(title)
        ax.set_ylabel(ylabel)
    deadline = data.deadline_limit.iloc[0]
    budget = data.budget_limit.iloc[0]
    axes[0, 0].axhline(deadline, color="#DC2626", linestyle="--", linewidth=1.8, label="Deadline fixo")
    axes[0, 1].axhline(budget, color="#DC2626", linestyle="--", linewidth=1.8, label="Budget fixo")

    feasibility = (
        data.assign(feasible_numeric=data.feasible.astype(int))
        .pivot(index="interference_percent", columns="algorithm", values="feasible_numeric")
        .reindex(columns=ORDER)
    )
    ax = axes[1, 1]
    annotations = feasibility.T.map(lambda value: "cumpre" if value else "viola").to_numpy()
    sns.heatmap(
        feasibility.T,
        cmap=sns.color_palette(["#FCA5A5", "#86EFAC"], as_cmap=True),
        vmin=0,
        vmax=1,
        cbar=False,
        linewidths=1,
        linecolor="white",
        annot=annotations,
        fmt="",
        ax=ax,
    )
    ax.set_title("Cumprimento simultâneo do SLA")
    ax.set_yticklabels([LABELS[item] for item in feasibility.columns], rotation=0)
    ax.set_ylabel("")
    ax.set_xticklabels([f"{int(value)}%" for value in feasibility.index], rotation=0)

    for ax in axes.flat[:3]:
        ax.set_xticks(range(10, 100, 10))
        ax.set_xticklabels([f"{value}%" for value in range(10, 100, 10)])
        ax.set_xlabel("Penalidade de interferência de software (%)")
    axes[1, 1].set_xlabel("Penalidade de interferência de software")
    handles, labels = axes[0, 0].get_legend_handles_labels()
    unique = dict(zip(labels, handles))
    fig.legend(unique.values(), unique.keys(), loc="lower center", ncol=4, frameon=False)
    fig.suptitle(
        "Impacto da interferência — ambiente híbrido heterogêneo\n"
        f"Montage 6.448 · {policy} · seed 1 · Beam 120 · SLA fixo",
        fontsize=20,
        y=0.995,
    )
    fig.tight_layout(rect=(0, 0.07, 1, 0.95))
    fig.savefig(output, dpi=180, bbox_inches="tight")
    plt.close(fig)


def plot_comparison(current: pd.DataFrame, baseline: pd.DataFrame, output: Path) -> None:
    sns.set_theme(style="whitegrid", context="talk")
    fig, axes = plt.subplots(2, 2, figsize=(17, 12))
    heft = current[current.algorithm == "heft_colocation"].set_index("interference_percent")
    series = [
        ("Upward Rank fixo", baseline[baseline.algorithm == "prism_cc_time"], "#60A5FA"),
        ("Tarefas prontas + lookahead", current[current.algorithm == "prism_cc_time"], "#1D4ED8"),
    ]
    ax = axes[0, 0]
    ax.plot(heft.index, heft.makespan, marker="o", color="#6B7280", label="HEFT")
    for label, frame, color in series:
        ax.plot(frame.interference_percent, frame.makespan, marker="o", color=color, label=label)
    ax.set_title("Objetivo tempo — makespan")
    ax.set_ylabel("Tempo (s)")
    ax.legend(fontsize=11)

    ax = axes[0, 1]
    for label, frame, color in series:
        indexed = frame.set_index("interference_percent")
        gain = 100 * (heft.makespan - indexed.makespan) / heft.makespan
        ax.plot(gain.index, gain, marker="o", color=color, label=label)
    ax.axhline(0, color="#6B7280", linestyle="--")
    ax.set_title("Ganho de makespan sobre o HEFT")
    ax.set_ylabel("Ganho (%)")

    for column, metric, title, ylabel in [
        (axes[1, 0], "makespan", "Objetivo custo — makespan", "Tempo (s)"),
        (axes[1, 1], "budget_used", "Objetivo custo — custo alcançado", "Custo (USD)"),
    ]:
        for label, data, color in [
            ("Upward Rank fixo", baseline[baseline.algorithm == "prism_cc_cost"], "#86EFAC"),
            ("Tarefas prontas + lookahead", current[current.algorithm == "prism_cc_cost"], "#15803D"),
        ]:
            column.plot(data.interference_percent, data[metric], marker="o", color=color, label=label)
        column.set_title(title)
        column.set_ylabel(ylabel)
        column.legend(fontsize=11)
    for ax in axes.flat:
        ax.set_xticks(range(10, 100, 10))
        ax.set_xticklabels([f"{value}%" for value in range(10, 100, 10)])
        ax.set_xlabel("Interferência de software")
    fig.suptitle("PRISM: ordem fixa × seleção de tarefas prontas com lookahead", fontsize=20)
    fig.tight_layout(rect=(0, 0, 1, 0.96))
    fig.savefig(output, dpi=180, bbox_inches="tight")
    plt.close(fig)


def write_readme(result_dir: Path, data: pd.DataFrame, policy: str, has_comparison: bool) -> None:
    deadline = data.deadline_limit.iloc[0]
    budget = data.budget_limit.iloc[0]
    text = f"""# Impacto da interferência de software

Experimento no ambiente híbrido heterogêneo com Montage 6.448, seed 1,
PRISM com **{policy}**, HEFT com co-location e Beam 120.

A penalidade por tarefa interferente sobreposta varia de 10% a 90%.
O conjunto de atividades interferentes permanece pareado e constante.
O SLA foi calibrado sem interferência e mantido fixo em todas as execuções:

- Deadline: {deadline:.6f} s
- Budget: US$ {budget:.6f}

![Impacto da interferência](figures/impacto-interferencia.png)
"""
    if has_comparison:
        text += "\n## Comparação com a ordem fixa\n\n![Comparação](figures/comparacao-upward-rank-ready-lookahead.png)\n"
    (result_dir / "README.md").write_text(text, encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("result_dir", type=Path)
    parser.add_argument("--baseline-dir", type=Path)
    args = parser.parse_args()
    data = load_sweep(args.result_dir)
    data.to_csv(args.result_dir / "interference_sweep_results.csv", index=False)
    figures = args.result_dir / "figures"
    figures.mkdir(exist_ok=True)
    policy = priority_label(args.result_dir)
    plot_sweep(data, figures / "impacto-interferencia.png", policy)
    if args.baseline_dir:
        baseline = load_sweep(args.baseline_dir)
        plot_comparison(
            data, baseline, figures / "comparacao-upward-rank-ready-lookahead.png"
        )
    write_readme(args.result_dir, data, policy, args.baseline_dir is not None)


if __name__ == "__main__":
    main()
