#!/usr/bin/env python3
import argparse
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd


LABELS = {
    "heft_colocation": "HEFT",
    "prism_cc_time": "PRISM-Time",
    "prism_cc_cost": "PRISM-Cost",
}
ORDER = ["heft_colocation", "prism_cc_time", "prism_cc_cost"]
COLORS = ["#64748b", "#2563eb", "#ea580c"]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("result_dir", type=Path)
    args = parser.parse_args()
    data = pd.read_csv(args.result_dir / "raw_results.csv").set_index("algorithm").loc[ORDER]
    labels = [LABELS[item] for item in ORDER]
    figures = args.result_dir / "figures"
    figures.mkdir(parents=True, exist_ok=True)

    fig, axes = plt.subplots(1, 3, figsize=(17, 6))
    metrics = [
        ("makespan", "Makespan", "segundos", "%.3f"),
        ("budget_used", "Custo", "US$", "%.4f"),
        ("algorithm_milliseconds", "Tempo do escalonador", "milissegundos", "%.0f"),
    ]
    for ax, (column, title, unit, fmt) in zip(axes, metrics):
        bars = ax.bar(labels, data[column], color=COLORS)
        ax.set_title(title, loc="left", fontsize=14, fontweight="bold")
        ax.set_ylabel(unit)
        ax.grid(axis="y", alpha=.2)
        ax.spines[["top", "right"]].set_visible(False)
        ax.bar_label(bars, fmt=fmt, padding=4)
        ax.margins(y=.15)
    fig.suptitle(
        "PRISM com caminho canônico corrigido — Montage 6.448, 50% de interferência",
        fontsize=17, fontweight="bold",
    )
    fig.tight_layout()
    output = figures / "resultado-caminho-canonico-corrigido.png"
    fig.savefig(output, dpi=180, bbox_inches="tight")
    plt.close(fig)

    (args.result_dir / "README.md").write_text(
        """# PRISM com caminho canônico corrigido

Montage com 6.448 atividades, ambiente híbrido heterogêneo, 50% de interferência, uma semente e Beam 120.

![Comparação entre HEFT, PRISM-Time e PRISM-Cost](figures/resultado-caminho-canonico-corrigido.png)
""",
        encoding="utf-8",
    )


if __name__ == "__main__":
    main()
