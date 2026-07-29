#!/usr/bin/env python3
import argparse
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd


COLORS = {
    "HEFT": "#64748b",
    "PRISM-Time anterior": "#60a5fa",
    "PRISM-Time adaptativo": "#2563eb",
    "PRISM-Cost anterior": "#fbbf24",
    "PRISM-Cost adaptativo": "#ea580c",
}


def load_rows(adaptive_dir: Path, previous_dir: Path) -> pd.DataFrame:
    adaptive = pd.read_csv(adaptive_dir / "raw_results.csv").set_index("algorithm")
    previous = pd.read_csv(previous_dir / "raw_results.csv").set_index("algorithm")
    rows = [
        ("HEFT", adaptive.loc["heft_colocation"]),
        ("PRISM-Time anterior", previous.loc["prism_cc_time"]),
        ("PRISM-Time adaptativo", adaptive.loc["prism_cc_time"]),
        ("PRISM-Cost anterior", previous.loc["prism_cc_cost"]),
        ("PRISM-Cost adaptativo", adaptive.loc["prism_cc_cost"]),
    ]
    return pd.DataFrame(
        [
            {
                "label": label,
                "makespan": row.makespan,
                "cost": row.budget_used,
                "interference": row.interference_time,
                "algorithm_seconds": row.algorithm_milliseconds / 1000,
            }
            for label, row in rows
        ]
    )


def style_axis(ax, title: str, ylabel: str) -> None:
    ax.set_title(title, loc="left", fontsize=14, fontweight="bold")
    ax.set_ylabel(ylabel)
    ax.grid(axis="y", alpha=.2)
    ax.spines[["top", "right"]].set_visible(False)
    ax.tick_params(axis="x", rotation=18)


def bar_plot(data: pd.DataFrame, metric: str, title: str, ylabel: str, output: Path) -> None:
    fig, ax = plt.subplots(figsize=(12, 6.5))
    bars = ax.bar(data.label, data[metric], color=[COLORS[x] for x in data.label])
    style_axis(ax, title, ylabel)
    ax.bar_label(bars, fmt="%.3f", padding=4, fontsize=9)
    ax.margins(y=.14)
    fig.tight_layout()
    fig.savefig(output, dpi=180, bbox_inches="tight")
    plt.close(fig)


def frontier_plot(data: pd.DataFrame, output: Path) -> None:
    fig, ax = plt.subplots(figsize=(10, 7))
    for row in data.itertuples():
        ax.scatter(row.makespan, row.cost, s=115, color=COLORS[row.label], zorder=3)
        ax.annotate(row.label, (row.makespan, row.cost), xytext=(7, 7),
                    textcoords="offset points", fontsize=9)
    ax.set_title("Relação custo × makespan — interferência de 50%", loc="left",
                 fontsize=14, fontweight="bold")
    ax.set_xlabel("Makespan (s) — menor é melhor")
    ax.set_ylabel("Custo (US$) — menor é melhor")
    ax.grid(alpha=.2)
    ax.spines[["top", "right"]].set_visible(False)
    fig.tight_layout()
    fig.savefig(output, dpi=180, bbox_inches="tight")
    plt.close(fig)


def summary_plot(data: pd.DataFrame, output: Path) -> None:
    fig, axes = plt.subplots(2, 2, figsize=(17, 12))
    for ax, metric, title, ylabel, fmt in [
        (axes[0, 0], "makespan", "Makespan", "segundos", "%.1f"),
        (axes[0, 1], "cost", "Custo", "US$", "%.3f"),
        (axes[1, 0], "interference", "Interferência acumulada", "segundos", "%.0f"),
        (axes[1, 1], "algorithm_seconds", "Tempo computacional", "segundos", "%.1f"),
    ]:
        bars = ax.bar(data.label, data[metric], color=[COLORS[x] for x in data.label])
        style_axis(ax, title, ylabel)
        ax.bar_label(bars, fmt=fmt, padding=3, fontsize=8)
        ax.margins(y=.15)
    fig.suptitle(
        "PRISM adaptativo × HEFT — Montage 6.448, ambiente híbrido heterogêneo, 50% de interferência",
        fontsize=17, fontweight="bold", y=.995,
    )
    fig.tight_layout()
    fig.savefig(output, dpi=180, bbox_inches="tight")
    plt.close(fig)


def write_readme(output_dir: Path) -> None:
    (output_dir.parent / "README.md").write_text(
        """# PRISM adaptativo × HEFT — interferência de 50%

Comparação do HEFT com as versões anterior e adaptativa do PRISM no Montage com 6.448 atividades, em ambiente híbrido heterogêneo e interferência de 50%.

![Resumo](figures/00-resumo-comparacao.png)

## Gráficos

- [Makespan](figures/01-comparacao-makespan.png)
- [Custo](figures/02-comparacao-custo.png)
- [Interferência acumulada](figures/03-comparacao-interferencia.png)
- [Relação custo × makespan](figures/04-fronteira-custo-makespan.png)
- [Tempo computacional](figures/05-tempo-computacional.png)
""",
        encoding="utf-8",
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("adaptive_dir", type=Path)
    parser.add_argument("previous_dir", type=Path)
    args = parser.parse_args()
    output = args.adaptive_dir / "figures"
    output.mkdir(parents=True, exist_ok=True)
    data = load_rows(args.adaptive_dir, args.previous_dir)
    summary_plot(data, output / "00-resumo-comparacao.png")
    bar_plot(data, "makespan", "Makespan — interferência de 50%", "segundos",
             output / "01-comparacao-makespan.png")
    bar_plot(data, "cost", "Custo — interferência de 50%", "US$",
             output / "02-comparacao-custo.png")
    bar_plot(data, "interference", "Interferência acumulada — 50%", "segundos",
             output / "03-comparacao-interferencia.png")
    frontier_plot(data, output / "04-fronteira-custo-makespan.png")
    bar_plot(data, "algorithm_seconds", "Tempo computacional do escalonador", "segundos",
             output / "05-tempo-computacional.png")
    write_readme(output)


if __name__ == "__main__":
    main()
