from __future__ import annotations

import json
import os
from pathlib import Path

import matplotlib

matplotlib.use("Agg")

import matplotlib.pyplot as plt
from matplotlib.lines import Line2D
from matplotlib.patches import Patch
import numpy as np
import pandas as pd
import seaborn as sns


SCENARIO_ORDER = [
    "cluster_homo",
    "cluster_hetero",
    "cloud_homo",
    "cloud_hetero",
    "hybrid_homo",
    "hybrid_hetero",
]
SCENARIO_LABELS = {
    "cluster_homo": "On-prem\nhomogêneo",
    "cluster_hetero": "On-prem\nheterogêneo",
    "cloud_homo": "Nuvem\nhomogênea",
    "cloud_hetero": "Nuvem\nheterogênea",
    "hybrid_homo": "Híbrido\nhomogêneo",
    "hybrid_hetero": "Híbrido\nheterogêneo",
}
ALGORITHM_ORDER = ["prism_cc_time", "prism_cc_cost", "heft_classic"]
ALGORITHM_LABELS = {
    "prism_cc_time": "PRISM-CC - Time",
    "prism_cc_cost": "PRISM-CC - Cost",
    "heft_classic": "HEFT clássico",
}
PALETTE = {"prism_cc_time": "#2878B5", "prism_cc_cost": "#59A14F", "heft_classic": "#E07A2D"}
EXPERIMENT_RESULT_DIR = os.environ.get(
    "EXPERIMENT_RESULT_DIR", "prism-cc-topology-order-exp-01"
)


def configure_style() -> None:
    sns.set_theme(style="whitegrid", context="notebook")
    plt.rcParams.update(
        {
            "figure.dpi": 120,
            "savefig.dpi": 180,
            "savefig.bbox": "tight",
            "font.family": "DejaVu Sans",
            "axes.titleweight": "bold",
        }
    )


def load_results(repo_root: Path) -> tuple[pd.DataFrame, dict]:
    results_dir = repo_root / "experiments" / "results" / EXPERIMENT_RESULT_DIR
    df = pd.read_csv(results_dir / "raw_results.csv")
    manifest = json.loads((results_dir / "manifest.json").read_text())
    df["algorithm_label"] = df["algorithm"].map(ALGORITHM_LABELS)
    df["scenario_label"] = df["scenario_id"].map(SCENARIO_LABELS)
    df["interference_percent"] = 100 * df["interference_time"] / (
        df["makespan"] - df["interference_time"]
    ).clip(lower=1e-9)
    return df, manifest


def validate_results(df: pd.DataFrame, manifest: dict) -> pd.DataFrame:
    expected = len(SCENARIO_ORDER) * len(ALGORITHM_ORDER) * len(manifest["interference_seeds"])
    assert len(df) == expected, f"esperadas {expected} linhas, encontradas {len(df)}"
    assert set(df["scenario_id"]) == set(SCENARIO_ORDER)
    assert set(df["algorithm"]) == set(ALGORITHM_ORDER)
    expected_selected = manifest["selected_activities"]
    assert (
        df["interference_activity_ids"]
        .str.split("|", regex=False)
        .str.len()
        .eq(expected_selected)
        .all()
    )
    counts = (
        df.groupby(["scenario_id", "algorithm"], observed=True)
        .size()
        .rename("runs")
        .reset_index()
    )
    assert counts["runs"].eq(len(manifest["interference_seeds"])).all()
    assert df["deadline_limit"].nunique() == 1
    assert df["budget_limit"].nunique() == 1
    heft = df[df["algorithm"] == "heft_classic"]
    assert np.isclose(df["deadline_limit"].iloc[0], heft["makespan"].mean(), atol=1e-6)
    assert np.isclose(df["budget_limit"].iloc[0], heft["budget_used"].mean(), atol=1e-6)
    return counts


def save(fig: plt.Figure, output_dir: Path, filename: str) -> None:
    fig.tight_layout()
    fig.savefig(output_dir / filename)
    plt.close(fig)


def add_sla_line(ax: plt.Axes, value: float, label: str) -> None:
    ax.axhline(value, color="#B33A3A", linestyle="--", linewidth=1.4, label=label)


def relabel_algorithm_legend(ax: plt.Axes, extra_labels: dict[str, str] | None = None) -> None:
    handles, labels = ax.get_legend_handles_labels()
    mapping = {**ALGORITHM_LABELS, **(extra_labels or {})}
    ax.legend(handles, [mapping.get(label, label) for label in labels], title="")


def plot_01_makespan(df: pd.DataFrame, manifest: dict, output: Path) -> None:
    fig, ax = plt.subplots(figsize=(12, 6))
    sns.boxplot(
        data=df, x="scenario_id", y="makespan", hue="algorithm",
        order=SCENARIO_ORDER, hue_order=ALGORITHM_ORDER, palette=PALETTE, ax=ax,
    )
    add_sla_line(ax, manifest["deadline_limit"], f"Deadline médio HEFT = {manifest['deadline_limit']:.3f} s")
    ax.set(
        title="Makespan por ambiente e algoritmo",
        xlabel="", ylabel="Makespan (s)",
    )
    ax.set_xticklabels([SCENARIO_LABELS[item] for item in SCENARIO_ORDER])
    relabel_algorithm_legend(ax)
    save(fig, output, "01-makespan-por-ambiente.png")


def plot_02_cost(df: pd.DataFrame, manifest: dict, output: Path) -> None:
    fig, ax = plt.subplots(figsize=(12, 6))
    sns.boxplot(
        data=df, x="scenario_id", y="budget_used", hue="algorithm",
        order=SCENARIO_ORDER, hue_order=ALGORITHM_ORDER, palette=PALETTE, ax=ax,
    )
    add_sla_line(ax, manifest["budget_limit"], f"Budget médio HEFT = {manifest['budget_limit']:.4f} USD")
    ax.set(
        title="Custo por ambiente e algoritmo",
        xlabel="", ylabel="Custo total (USD)",
    )
    ax.set_xticklabels([SCENARIO_LABELS[item] for item in SCENARIO_ORDER])
    relabel_algorithm_legend(ax)
    save(fig, output, "02-custo-por-ambiente.png")


def plot_03_feasibility(df: pd.DataFrame, output: Path) -> None:
    feasible = (
        df.groupby(["scenario_id", "algorithm"], observed=True)["feasible"]
        .mean().mul(100).reset_index()
    )
    fig, ax = plt.subplots(figsize=(12, 5.5))
    sns.barplot(
        data=feasible, x="scenario_id", y="feasible", hue="algorithm",
        order=SCENARIO_ORDER, hue_order=ALGORITHM_ORDER, palette=PALETTE, ax=ax,
    )
    for container in ax.containers:
        ax.bar_label(container, fmt="%.1f%%", padding=3, rotation=90, fontsize=8)
    ax.set(title="Execuções que respeitam budget e deadline", xlabel="", ylabel="Factibilidade (%)", ylim=(0, 105))
    ax.set_xticklabels([SCENARIO_LABELS[item] for item in SCENARIO_ORDER])
    relabel_algorithm_legend(ax)
    save(fig, output, "03-factibilidade.png")


def plot_04_cost_makespan(df: pd.DataFrame, manifest: dict, output: Path) -> None:
    fig, axes = plt.subplots(2, 3, figsize=(15, 9), sharex=True, sharey=True)
    for ax, scenario in zip(axes.flat, SCENARIO_ORDER):
        subset = df[df["scenario_id"] == scenario]
        sns.scatterplot(
            data=subset, x="budget_used", y="makespan", hue="algorithm",
            style="algorithm", hue_order=ALGORITHM_ORDER, palette=PALETTE, s=55, alpha=.75, ax=ax,
        )
        ax.axvline(manifest["budget_limit"], color="#B33A3A", linestyle="--", linewidth=1)
        ax.axhline(manifest["deadline_limit"], color="#B33A3A", linestyle="--", linewidth=1)
        ax.set_title(SCENARIO_LABELS[scenario].replace("\n", " "))
        if ax.get_legend():
            ax.get_legend().remove()
    handles, labels = axes.flat[0].get_legend_handles_labels()
    fig.legend(handles, [ALGORITHM_LABELS.get(x, x) for x in labels], loc="upper center", ncol=2)
    fig.suptitle("Trade-off custo × makespan — limites definidos pelas médias globais do HEFT", y=1.02, fontweight="bold")
    save(fig, output, "04-custo-versus-makespan.png")


def paired_deltas(df: pd.DataFrame) -> pd.DataFrame:
    pivot = df.pivot(index=["scenario_id", "interference_seed"], columns="algorithm", values=["makespan", "budget_used"])
    rows = []
    for (scenario_id, seed), values in pivot.iterrows():
        for algorithm in ["prism_cc_time", "prism_cc_cost"]:
            rows.append(
                {
                    "scenario_id": scenario_id,
                    "interference_seed": seed,
                    "algorithm": algorithm,
                    "delta_makespan": values[("makespan", "heft_classic")] - values[("makespan", algorithm)],
                    "delta_budget": values[("budget_used", "heft_classic")] - values[("budget_used", algorithm)],
                }
            )
    return pd.DataFrame(rows)


def plot_05_gain(df: pd.DataFrame, output: Path) -> None:
    deltas = paired_deltas(df)
    fig, axes = plt.subplots(1, 2, figsize=(14, 5.8))
    for ax, metric, label in [
        (axes[0], "delta_makespan", "HEFT − PRISM-CC (s)"),
        (axes[1], "delta_budget", "HEFT − PRISM-CC (custo)"),
    ]:
        sns.boxplot(
            data=deltas, x="scenario_id", y=metric, hue="algorithm",
            order=SCENARIO_ORDER, hue_order=["prism_cc_time", "prism_cc_cost"], palette=PALETTE, ax=ax,
        )
        ax.axhline(0, color="#333333", linewidth=1)
        ax.set(xlabel="", ylabel=label)
        ax.set_xticklabels([SCENARIO_LABELS[item] for item in SCENARIO_ORDER], rotation=15)
    axes[0].set_title("Ganho pareado do PRISM-CC em makespan")
    axes[1].set_title("Ganho pareado do PRISM-CC em custo")
    for ax in axes:
        handles, labels = ax.get_legend_handles_labels()
        ax.legend(handles, [ALGORITHM_LABELS[item] for item in labels], title="")
    fig.suptitle("Valores positivos favorecem a respectiva variante PRISM-CC", y=1.02, fontweight="bold")
    save(fig, output, "05-ganho-prism-cc-sobre-heft.png")


def faceted_scatter(df: pd.DataFrame, x: str, filename: str, title: str, xlabel: str, output: Path) -> None:
    fig, axes = plt.subplots(2, 3, figsize=(15, 9), sharey=False)
    for ax, scenario in zip(axes.flat, SCENARIO_ORDER):
        subset = df[df["scenario_id"] == scenario]
        for algorithm in ALGORITHM_ORDER:
            sns.regplot(
                data=subset[subset["algorithm"] == algorithm], x=x, y="makespan",
                scatter_kws={"alpha": .55, "s": 28, "color": PALETTE[algorithm]},
                line_kws={"color": PALETTE[algorithm]}, ax=ax,
            )
        ax.set_title(SCENARIO_LABELS[scenario].replace("\n", " "))
        ax.set(xlabel=xlabel, ylabel="Makespan (s)")
    fig.suptitle(title, y=1.01, fontweight="bold")
    save(fig, output, filename)


def plot_08_interference_distribution(df: pd.DataFrame, output: Path) -> None:
    fig, ax = plt.subplots(figsize=(12, 6))
    sns.boxplot(
        data=df, x="scenario_id", y="interference_time", hue="algorithm",
        order=SCENARIO_ORDER, hue_order=ALGORITHM_ORDER, palette=PALETTE, ax=ax,
    )
    ax.set(title="Tempo total adicionado pela interferência", xlabel="", ylabel="Tempo de interferência (s)")
    ax.set_xticklabels([SCENARIO_LABELS[item] for item in SCENARIO_ORDER])
    relabel_algorithm_legend(ax)
    save(fig, output, "08-tempo-de-interferencia.png")


def machine_family(machine_id: str) -> str:
    if machine_id.startswith("bora"):
        return "Bora"
    if machine_id.startswith("diablo"):
        return "Diablo"
    if machine_id.startswith("h3-"):
        return "H3"
    if machine_id.startswith("h4d-"):
        return "H4D"
    return machine_id


def expand_json_metric(df: pd.DataFrame, column: str, value_name: str) -> pd.DataFrame:
    rows = []
    for record in df.itertuples():
        values = json.loads(getattr(record, column))
        for machine_id, value in values.items():
            rows.append(
                {
                    "scenario_id": record.scenario_id,
                    "algorithm": record.algorithm,
                    "interference_seed": record.interference_seed,
                    "machine_id": machine_id,
                    "family": machine_family(machine_id),
                    value_name: value,
                }
            )
    return pd.DataFrame(rows)


def plot_09_utilization(df: pd.DataFrame, output: Path) -> None:
    usage = expand_json_metric(df, "machine_utilization", "utilization")
    table = usage.pivot_table(index="machine_id", columns=["scenario_id", "algorithm"], values="utilization", aggfunc="mean")
    columns = [(scenario, algorithm) for scenario in SCENARIO_ORDER for algorithm in ALGORITHM_ORDER if (scenario, algorithm) in table.columns]
    table = table.reindex(columns=columns).fillna(0)
    table.columns = [f"{SCENARIO_LABELS[s].replace(chr(10), ' ')}\n{ALGORITHM_LABELS[a]}" for s, a in table.columns]
    fig, ax = plt.subplots(figsize=(16, max(6, .38 * len(table))))
    sns.heatmap(table * 100, cmap="YlGnBu", linewidths=.4, annot=True, fmt=".1f", cbar_kws={"label": "Utilização média (%)"}, ax=ax)
    ax.set(title="Utilização média por máquina", xlabel="", ylabel="Máquina")
    save(fig, output, "09-heatmap-utilizacao.png")


def plot_10_distribution(df: pd.DataFrame, output: Path) -> None:
    distribution = expand_json_metric(df, "machine_distribution", "activities")
    grouped = distribution.groupby(["scenario_id", "algorithm", "family"], observed=True)["activities"].mean().reset_index()
    grouped["share"] = grouped["activities"] / grouped.groupby(["scenario_id", "algorithm"], observed=True)["activities"].transform("sum")
    families = ["Bora", "Diablo", "H3", "H4D"]
    labels, values = [], []
    for scenario in SCENARIO_ORDER:
        for algorithm in ALGORITHM_ORDER:
            labels.append(f"{SCENARIO_LABELS[scenario].replace(chr(10), ' ')}\n{ALGORITHM_LABELS[algorithm]}")
            subset = grouped[(grouped.scenario_id == scenario) & (grouped.algorithm == algorithm)]
            values.append([float(subset.loc[subset.family == family, "share"].sum()) for family in families])
    values = np.asarray(values)
    fig, ax = plt.subplots(figsize=(16, 6))
    bottom = np.zeros(len(labels))
    colors = ["#4C78A8", "#72B7B2", "#F58518", "#E45756"]
    for index, family in enumerate(families):
        ax.bar(labels, values[:, index] * 100, bottom=bottom * 100, label=family, color=colors[index])
        bottom += values[:, index]
    ax.set(title="Distribuição média das atividades por família de máquina", xlabel="", ylabel="Atividades alocadas (%)", ylim=(0, 100))
    ax.tick_params(axis="x", rotation=35)
    ax.legend(title="Família", ncol=4)
    save(fig, output, "10-distribuicao-atividades.png")


def plot_11_algorithm_time(df: pd.DataFrame, output: Path) -> None:
    fig, ax = plt.subplots(figsize=(12, 6))
    sns.boxplot(
        data=df, x="scenario_id", y="algorithm_milliseconds", hue="algorithm",
        order=SCENARIO_ORDER, hue_order=ALGORITHM_ORDER, palette=PALETTE, ax=ax,
    )
    ax.set_yscale("log")
    ax.set(title="Tempo computacional dos algoritmos", xlabel="", ylabel="Tempo (ms, escala log)")
    ax.set_xticklabels([SCENARIO_LABELS[item] for item in SCENARIO_ORDER])
    relabel_algorithm_legend(ax)
    save(fig, output, "11-tempo-computacional.png")


def plot_12_seed_stability(df: pd.DataFrame, output: Path) -> None:
    fig, axes = plt.subplots(2, 3, figsize=(16, 9), sharex=True)
    for ax, scenario in zip(axes.flat, SCENARIO_ORDER):
        subset = df[df.scenario_id == scenario]
        for algorithm in ALGORITHM_ORDER:
            series = subset[subset.algorithm == algorithm].sort_values("interference_seed")
            ax.plot(series.interference_seed, series.makespan, marker="o", markersize=2.5, linewidth=1.2, color=PALETTE[algorithm], label=ALGORITHM_LABELS[algorithm])
        ax.set_title(SCENARIO_LABELS[scenario].replace("\n", " "))
        ax.set(xlabel="Semente", ylabel="Makespan (s)")
    handles, labels = axes.flat[0].get_legend_handles_labels()
    fig.legend(handles, labels, loc="upper center", ncol=2)
    fig.suptitle("Estabilidade do makespan entre as 30 sementes", y=1.01, fontweight="bold")
    save(fig, output, "12-estabilidade-por-semente.png")


def aggregate_environment_statistics(df: pd.DataFrame) -> pd.DataFrame:
    rows = []
    for (scenario, algorithm), group in df.groupby(
        ["scenario_id", "algorithm"], observed=True
    ):
        row = {"scenario_id": scenario, "algorithm": algorithm, "runs": len(group)}
        for metric in ["makespan", "budget_used"]:
            values = group[metric].astype(float)
            standard_error = values.std(ddof=1) / np.sqrt(len(values)) if len(values) > 1 else 0
            row[f"{metric}_mean"] = values.mean()
            row[f"{metric}_median"] = values.median()
            row[f"{metric}_stddev"] = values.std(ddof=1) if len(values) > 1 else 0
            row[f"{metric}_ci95"] = 1.96 * standard_error
        rows.append(row)
    return pd.DataFrame(rows)


def plot_14_aggregate_forest(
    aggregate: pd.DataFrame,
    metric: str,
    filename: str,
    title: str,
    xlabel: str,
    output: Path,
) -> None:
    fig, axes = plt.subplots(1, 3, figsize=(16, 6), sharey=True)
    y = np.arange(len(SCENARIO_ORDER))
    for ax, algorithm in zip(axes, ALGORITHM_ORDER):
        subset = (
            aggregate[aggregate["algorithm"] == algorithm]
            .set_index("scenario_id")
            .reindex(SCENARIO_ORDER)
        )
        mean = subset[f"{metric}_mean"].to_numpy()
        median = subset[f"{metric}_median"].to_numpy()
        ci95 = subset[f"{metric}_ci95"].to_numpy()
        ax.errorbar(
            mean, y, xerr=ci95, fmt="o", markersize=7, capsize=4,
            color=PALETTE[algorithm], linewidth=1.5, label="Média ± IC 95%",
        )
        ax.scatter(
            median, y, marker="D", s=34, facecolors="none",
            edgecolors=PALETTE[algorithm], linewidths=1.3, label="Mediana",
        )
        for value, position in zip(mean, y):
            ax.annotate(
                f"{value:.3f}", (value, position), xytext=(6, -10),
                textcoords="offset points", fontsize=8,
            )
        ax.set_title(ALGORITHM_LABELS[algorithm])
        ax.set_xlabel(xlabel)
        ax.set_yticks(y)
        ax.set_yticklabels(
            [SCENARIO_LABELS[item].replace("\n", " ") for item in SCENARIO_ORDER]
        )
        ax.invert_yaxis()
        ax.legend(loc="best", fontsize=8)
    fig.suptitle(title, y=1.02)
    save(fig, output, filename)


def plot_16_aggregate_cost_makespan(
    aggregate: pd.DataFrame, output: Path
) -> None:
    fig, axes = plt.subplots(2, 3, figsize=(15, 9))
    markers = {"prism_cc_time": "o", "prism_cc_cost": "s", "heft_classic": "X"}
    for ax, scenario in zip(axes.flat, SCENARIO_ORDER):
        subset = aggregate[aggregate["scenario_id"] == scenario]
        for _, row in subset.iterrows():
            algorithm = row["algorithm"]
            ax.errorbar(
                row["budget_used_mean"], row["makespan_mean"],
                xerr=row["budget_used_ci95"], yerr=row["makespan_ci95"],
                fmt=markers[algorithm], markersize=8, capsize=4,
                color=PALETTE[algorithm], linewidth=1.4,
                label=ALGORITHM_LABELS[algorithm],
            )
        ax.set_title(SCENARIO_LABELS[scenario].replace("\n", " "))
        ax.set_xlabel("Custo médio (USD)")
        ax.set_ylabel("Makespan médio (s)")
    handles, labels = axes.flat[0].get_legend_handles_labels()
    fig.legend(
        handles, labels, loc="upper center", ncol=3, frameon=False,
        bbox_to_anchor=(0.5, 1.035),
    )
    fig.suptitle(
        "Relação agregada entre custo e makespan — média ± IC 95%", y=1.085
    )
    save(fig, output, "16-relacao-agregada-custo-makespan.png")


def plot_17_winner_scorecard(
    aggregate: pd.DataFrame, output: Path
) -> pd.DataFrame:
    prism_algorithms = ["prism_cc_time", "prism_cc_cost"]
    comparison_rows = []
    matrices = {}
    annotations = {}
    for metric in ["makespan", "budget_used"]:
        values = np.zeros((len(SCENARIO_ORDER), len(prism_algorithms)))
        labels = np.empty_like(values, dtype=object)
        for row_index, scenario in enumerate(SCENARIO_ORDER):
            scenario_data = aggregate[aggregate["scenario_id"] == scenario].set_index(
                "algorithm"
            )
            heft_value = scenario_data.loc["heft_classic", f"{metric}_mean"]
            for column_index, algorithm in enumerate(prism_algorithms):
                prism_value = scenario_data.loc[algorithm, f"{metric}_mean"]
                if np.isclose(heft_value, 0) and np.isclose(prism_value, 0):
                    gain = 0.0
                    winner = "Empate"
                    label = "EMPATE\n0 × 0"
                elif np.isclose(heft_value, 0):
                    gain = -100.0
                    winner = "HEFT"
                    label = "HEFT\nreferência = 0"
                else:
                    gain = 100 * (heft_value - prism_value) / heft_value
                    winner = "PRISM-CC" if gain > 0 else ("HEFT" if gain < 0 else "Empate")
                    label = (
                        f"PRISM-CC\n+{gain:.1f}%"
                        if gain > 0
                        else (f"HEFT\n+{-gain:.1f}%" if gain < 0 else "EMPATE")
                    )
                values[row_index, column_index] = gain
                labels[row_index, column_index] = label
                comparison_rows.append(
                    {
                        "scenario_id": scenario,
                        "variant": algorithm,
                        "metric": metric,
                        "prism_mean": prism_value,
                        "heft_mean": heft_value,
                        "prism_gain_percent": gain,
                        "winner": winner,
                    }
                )
        matrices[metric] = values
        annotations[metric] = labels

    fig, axes = plt.subplots(1, 2, figsize=(12.5, 7), sharey=True)
    titles = {
        "makespan": "Makespan médio",
        "budget_used": "Custo médio",
    }
    for ax, metric in zip(axes, ["makespan", "budget_used"]):
        sns.heatmap(
            matrices[metric], annot=annotations[metric], fmt="",
            cmap="RdYlGn", center=0, vmin=-100, vmax=100,
            linewidths=1, linecolor="white",
            xticklabels=["PRISM-CC - Time", "PRISM-CC - Cost"],
            yticklabels=[
                SCENARIO_LABELS[item].replace("\n", " ") for item in SCENARIO_ORDER
            ],
            cbar=metric == "budget_used",
            cbar_kws={"label": "Ganho do PRISM-CC sobre o HEFT (%)"},
            ax=ax,
        )
        ax.set_title(titles[metric])
        ax.set_xlabel("")
        ax.set_ylabel("")
        ax.tick_params(axis="x", rotation=0)
        ax.tick_params(axis="y", rotation=0)
    fig.suptitle(
        "Quem ganha? PRISM-CC versus HEFT — médias das 30 execuções", y=1.02
    )
    fig.text(
        0.5, 0.005,
        "Verde: PRISM-CC apresenta menor valor. Vermelho: HEFT apresenta menor valor.",
        ha="center", fontsize=9,
    )
    save(fig, output, "17-placar-prism-cc-versus-heft.png")
    return pd.DataFrame(comparison_rows)


def plot_18_bars_and_gain_lines(
    aggregate: pd.DataFrame, output: Path
) -> None:
    fig, axes = plt.subplots(2, 1, figsize=(16, 11), sharex=True)
    x = np.arange(len(SCENARIO_ORDER))
    bar_width = 0.23
    offsets = {
        "prism_cc_time": -bar_width,
        "prism_cc_cost": 0,
        "heft_classic": bar_width,
    }
    metrics = [
        ("makespan", "Makespan médio (s)", True),
        ("budget_used", "Custo médio (USD)", False),
    ]
    for ax, (metric, ylabel, logarithmic) in zip(axes, metrics):
        indexed = aggregate.set_index(["scenario_id", "algorithm"])
        metric_values = {}
        for algorithm in ALGORITHM_ORDER:
            means = np.array([
                indexed.loc[(scenario, algorithm), f"{metric}_mean"]
                for scenario in SCENARIO_ORDER
            ])
            ci95 = np.array([
                indexed.loc[(scenario, algorithm), f"{metric}_ci95"]
                for scenario in SCENARIO_ORDER
            ])
            metric_values[algorithm] = means
            bars = ax.bar(
                x + offsets[algorithm], means, bar_width,
                yerr=ci95, capsize=3, color=PALETTE[algorithm], alpha=0.82,
                label=ALGORITHM_LABELS[algorithm],
            )
            for bar, value in zip(bars, means):
                text = f"{value:,.0f}" if metric == "makespan" else f"{value:.2f}"
                if np.isclose(value, 0):
                    ax.annotate(
                        "0", (bar.get_x() + bar.get_width() / 2, 0),
                        xytext=(0, 4), textcoords="offset points",
                        ha="center", va="bottom", fontsize=8,
                    )
                else:
                    ax.annotate(
                        text, (bar.get_x() + bar.get_width() / 2, value),
                        xytext=(0, 5), textcoords="offset points",
                        ha="center", va="bottom", fontsize=8, rotation=90,
                    )
        if logarithmic:
            ax.set_yscale("log")
        ax.set_ylabel(ylabel)
        ax.set_title(
            "Tempo: valores médios, IC 95% e ganho relativo"
            if metric == "makespan"
            else "Custo: valores médios, IC 95% e ganho relativo"
        )
        ax.grid(axis="x", visible=False)

        gain_axis = ax.twinx()
        heft = metric_values["heft_classic"]
        for algorithm, linestyle, marker in [
            ("prism_cc_time", "-", "o"),
            ("prism_cc_cost", "--", "s"),
        ]:
            prism = metric_values[algorithm]
            gains = np.full(len(prism), np.nan)
            nonzero = ~np.isclose(heft, 0)
            gains[nonzero] = 100 * (heft[nonzero] - prism[nonzero]) / heft[nonzero]
            gain_axis.plot(
                x, gains, color=PALETTE[algorithm], linestyle=linestyle,
                marker=marker, linewidth=2, markersize=6,
            )
            for position, gain in zip(x, gains):
                if np.isfinite(gain):
                    gain_axis.annotate(
                        f"{gain:+.1f}%", (position, gain),
                        xytext=(0, 7 if gain >= 0 else -13),
                        textcoords="offset points", ha="center", fontsize=8,
                        color=PALETTE[algorithm],
                    )
        gain_axis.axhline(0, color="#555555", linewidth=1, linestyle=":")
        gain_axis.set_ylabel("Ganho do PRISM-CC sobre o HEFT (%)")
        finite_gains = []
        for algorithm in ["prism_cc_time", "prism_cc_cost"]:
            prism = metric_values[algorithm]
            valid = ~np.isclose(heft, 0)
            finite_gains.extend(
                (100 * (heft[valid] - prism[valid]) / heft[valid]).tolist()
            )
        gain_axis.set_ylim(min(-10, min(finite_gains) - 10), max(105, max(finite_gains) + 10))

    axes[-1].set_xticks(x)
    axes[-1].set_xticklabels(
        [SCENARIO_LABELS[item] for item in SCENARIO_ORDER]
    )
    legend_items = [
        Patch(facecolor=PALETTE[algorithm], alpha=0.82, label=ALGORITHM_LABELS[algorithm])
        for algorithm in ALGORITHM_ORDER
    ] + [
        Line2D([0], [0], color=PALETTE["prism_cc_time"], marker="o", label="Ganho Time"),
        Line2D([0], [0], color=PALETTE["prism_cc_cost"], marker="s", linestyle="--", label="Ganho Cost"),
    ]
    fig.legend(
        handles=legend_items, loc="upper center", ncol=5, frameon=False,
        bbox_to_anchor=(0.5, 1.015),
    )
    fig.suptitle(
        "PRISM-CC versus HEFT por ambiente — barras agregadas e linhas de ganho",
        y=1.055,
    )
    save(fig, output, "18-barras-e-linhas-prism-cc-versus-heft.png")


def multidimensional_summary(
    df: pd.DataFrame, aggregate: pd.DataFrame, manifest: dict
) -> pd.DataFrame:
    task_count = manifest.get("task_count", 58)
    utilization_rows = []
    for record in df.itertuples():
        values = np.array(list(json.loads(record.machine_utilization).values()))
        utilization_rows.append(
            {
                "scenario_id": record.scenario_id,
                "algorithm": record.algorithm,
                "interference_seed": record.interference_seed,
                "utilization_mean": values.mean(),
                "utilization_imbalance": values.std(),
                "machines_used": int((values > 0).sum()),
            }
        )
    utilization = pd.DataFrame(utilization_rows).groupby(
        ["scenario_id", "algorithm"], observed=True
    ).mean(numeric_only=True).reset_index()
    interference = df.groupby(
        ["scenario_id", "algorithm"], observed=True
    ).agg(
        interference_time_mean=("interference_time", "mean"),
        interference_pairs_mean=("interference_pairs", "mean"),
    ).reset_index()
    summary = aggregate.merge(utilization, on=["scenario_id", "algorithm"]).merge(
        interference, on=["scenario_id", "algorithm"]
    )
    summary["interference_per_task"] = summary["interference_time_mean"] / task_count
    summary["makespan_cv_percent"] = (
        100 * summary["makespan_stddev"] / summary["makespan_mean"].clip(lower=1e-9)
    )
    for scenario in SCENARIO_ORDER:
        mask = summary["scenario_id"] == scenario
        heft = summary[mask & (summary["algorithm"] == "heft_classic")].iloc[0]
        prism_mask = mask & summary["algorithm"].isin(["prism_cc_time", "prism_cc_cost"])
        summary.loc[prism_mask, "makespan_gain_percent"] = (
            100
            * (heft["makespan_mean"] - summary.loc[prism_mask, "makespan_mean"])
            / max(heft["makespan_mean"], 1e-9)
        )
        if np.isclose(heft["budget_used_mean"], 0):
            summary.loc[prism_mask, "budget_gain_percent"] = 0
        else:
            summary.loc[prism_mask, "budget_gain_percent"] = (
                100
                * (heft["budget_used_mean"] - summary.loc[prism_mask, "budget_used_mean"])
                / heft["budget_used_mean"]
            )
    return summary


def plot_19_multidimensional_gain(
    summary: pd.DataFrame, output: Path
) -> None:
    prism_algorithms = ["prism_cc_time", "prism_cc_cost"]
    fig, axes = plt.subplots(1, 2, figsize=(14, 6.5), sharex=True, sharey=True)
    prism = summary[summary.algorithm.isin(prism_algorithms)]
    color_min, color_max = prism.interference_per_task.min(), prism.interference_per_task.max()
    scatter = None
    for ax, algorithm in zip(axes, prism_algorithms):
        data = prism[prism.algorithm == algorithm].set_index("scenario_id").reindex(SCENARIO_ORDER)
        sizes = 100 + 1400 * data.utilization_imbalance
        scatter = ax.scatter(
            data.budget_gain_percent, data.makespan_gain_percent,
            s=sizes, c=data.interference_per_task, cmap="viridis",
            vmin=color_min, vmax=color_max, alpha=0.85,
            edgecolor="black", linewidth=0.6,
        )
        for number, (scenario, row) in enumerate(data.iterrows(), start=1):
            ax.annotate(
                str(number),
                (row.budget_gain_percent, row.makespan_gain_percent),
                ha="center", va="center", fontsize=8, fontweight="bold",
            )
        ax.axhline(0, color="#555555", linestyle=":", linewidth=1)
        ax.axvline(0, color="#555555", linestyle=":", linewidth=1)
        ax.set_title(ALGORITHM_LABELS[algorithm])
        ax.set_xlabel("Redução média de custo sobre o HEFT (%)")
        ax.set_ylabel("Redução média de makespan sobre o HEFT (%)")
    colorbar = fig.colorbar(scatter, ax=axes, shrink=0.85)
    colorbar.set_label("Interferência média por atividade (s)")
    scenario_handles = [
        Line2D(
            [], [], linestyle="none", marker=f"${number}$", color="#222222",
            markersize=9,
            label=SCENARIO_LABELS[scenario].replace("\n", " "),
        )
        for number, scenario in enumerate(SCENARIO_ORDER, start=1)
    ]
    fig.legend(
        handles=scenario_handles, title="Ambientes", loc="lower center",
        ncol=3, bbox_to_anchor=(0.47, -0.08), frameon=False,
    )
    fig.suptitle(
        "Mapa multidimensional de ganhos — tamanho da bolha = desequilíbrio de utilização",
        y=1.02,
    )
    fig.subplots_adjust(right=0.88, bottom=0.18, wspace=0.18)
    fig.savefig(output / "19-mapa-multidimensional-ganhos.png", dpi=180, bbox_inches="tight")
    plt.close(fig)


def plot_20_price_of_savings(df: pd.DataFrame, output: Path) -> pd.DataFrame:
    prism = df[df.algorithm.isin(["prism_cc_time", "prism_cc_cost"])]
    pivot = prism.pivot(
        index=["scenario_id", "interference_seed"],
        columns="algorithm", values=["makespan", "budget_used"],
    )
    rows = []
    for scenario, group in pivot.groupby(level=0):
        extra_time = group[("makespan", "prism_cc_cost")] - group[("makespan", "prism_cc_time")]
        saved_cost = group[("budget_used", "prism_cc_time")] - group[("budget_used", "prism_cc_cost")]
        rows.append(
            {
                "scenario_id": scenario,
                "extra_time_mean": extra_time.mean(),
                "extra_time_ci95": 1.96 * extra_time.std(ddof=1) / np.sqrt(len(extra_time)),
                "saved_cost_mean": saved_cost.mean(),
                "saved_cost_ci95": 1.96 * saved_cost.std(ddof=1) / np.sqrt(len(saved_cost)),
                "extra_time_cv": extra_time.std(ddof=1) / max(abs(extra_time.mean()), 1e-9),
                "seconds_per_dollar": extra_time.mean() / saved_cost.mean() if saved_cost.mean() > 0 else np.nan,
            }
        )
    tradeoff = pd.DataFrame(rows).set_index("scenario_id").reindex(SCENARIO_ORDER).reset_index()
    fig, ax = plt.subplots(figsize=(11, 7))
    sizes = 100 + 700 * tradeoff.extra_time_cv.clip(upper=1)
    ax.errorbar(
        tradeoff.saved_cost_mean, tradeoff.extra_time_mean,
        xerr=tradeoff.saved_cost_ci95, yerr=tradeoff.extra_time_ci95,
        fmt="none", ecolor="#777777", capsize=4, linewidth=1.2,
    )
    ax.scatter(
        tradeoff.saved_cost_mean, tradeoff.extra_time_mean,
        s=sizes, c=np.arange(len(tradeoff)), cmap="tab10",
        edgecolor="black", linewidth=0.6,
    )
    for number, row in enumerate(tradeoff.itertuples(), start=1):
        ax.annotate(
            str(number), (row.saved_cost_mean, row.extra_time_mean),
            ha="center", va="center", fontsize=8, fontweight="bold",
        )
        if np.isfinite(row.seconds_per_dollar):
            label = f"{row.seconds_per_dollar:.0f} s/US$"
        else:
            label = "sem economia monetária"
        ax.annotate(
            label, (row.saved_cost_mean, row.extra_time_mean),
            xytext=(6, 5), textcoords="offset points", fontsize=8,
        )
    scenario_handles = [
        Line2D(
            [], [], linestyle="none", marker=f"${number}$", color="#222222",
            markersize=9,
            label=SCENARIO_LABELS[scenario].replace("\n", " "),
        )
        for number, scenario in enumerate(SCENARIO_ORDER, start=1)
    ]
    ax.legend(
        handles=scenario_handles, title="Ambientes", ncol=2,
        loc="upper left", frameon=True,
    )
    ax.axhline(0, color="#555555", linestyle=":", linewidth=1)
    ax.axvline(0, color="#555555", linestyle=":", linewidth=1)
    ax.set(
        title="Preço da economia ao escolher PRISM-CC Cost",
        xlabel="Custo médio economizado em relação ao PRISM-CC Time (USD)",
        ylabel="Makespan médio adicional (s)",
    )
    save(fig, output, "20-preco-da-economia.png")
    return tradeoff


def plot_21_allocation_explains_result(
    df: pd.DataFrame, aggregate: pd.DataFrame, output: Path
) -> None:
    distribution = expand_json_metric(
        df[df.algorithm.isin(["prism_cc_time", "prism_cc_cost"])],
        "machine_distribution", "activities",
    )
    grouped = distribution.groupby(
        ["scenario_id", "algorithm", "family"], observed=True
    ).activities.mean().reset_index()
    grouped["share"] = grouped.activities / grouped.groupby(
        ["scenario_id", "algorithm"], observed=True
    ).activities.transform("sum")
    families = ["Bora", "Diablo", "H3", "H4D"]
    colors = ["#4C78A8", "#72B7B2", "#F58518", "#E45756"]
    fig, axes = plt.subplots(1, 2, figsize=(16, 7), sharey=True)
    for ax, algorithm in zip(axes, ["prism_cc_time", "prism_cc_cost"]):
        bottom = np.zeros(len(SCENARIO_ORDER))
        for family, color in zip(families, colors):
            values = []
            for scenario in SCENARIO_ORDER:
                value = grouped[
                    (grouped.scenario_id == scenario)
                    & (grouped.algorithm == algorithm)
                    & (grouped.family == family)
                ].share.sum()
                values.append(value)
            ax.bar(
                np.arange(len(SCENARIO_ORDER)), np.array(values) * 100,
                bottom=bottom * 100, color=color, label=family,
            )
            bottom += values
        metric = aggregate[aggregate.algorithm == algorithm].set_index(
            "scenario_id"
        ).reindex(SCENARIO_ORDER)
        line_axis = ax.twinx()
        max_cost = max(metric.budget_used_mean.max(), 1e-9)
        sizes = 70 + 260 * metric.budget_used_mean / max_cost
        line_axis.plot(
            np.arange(len(SCENARIO_ORDER)), metric.makespan_mean,
            color="#222222", linewidth=1.8, marker="o",
        )
        line_axis.scatter(
            np.arange(len(SCENARIO_ORDER)), metric.makespan_mean,
            s=sizes, color="#222222", alpha=0.65,
        )
        for position, (_, row) in enumerate(metric.iterrows()):
            line_axis.annotate(
                f"{row.makespan_mean:.0f}s\nUS$ {row.budget_used_mean:.2f}",
                (position, row.makespan_mean), xytext=(0, 8),
                textcoords="offset points", ha="center", fontsize=8,
            )
        ax.set_title(ALGORITHM_LABELS[algorithm])
        ax.set_ylabel("Atividades por família de máquina (%)")
        line_axis.set_ylabel("Makespan médio (s)")
        ax.set_xticks(np.arange(len(SCENARIO_ORDER)))
        ax.set_xticklabels(
            [SCENARIO_LABELS[item] for item in SCENARIO_ORDER], rotation=20
        )
    axes[0].legend(title="Família", ncol=4, loc="upper center")
    fig.suptitle(
        "Como a alocação das atividades explica makespan e custo", y=1.02
    )
    save(fig, output, "21-alocacao-explica-resultado.png")


def plot_22_risk_performance(
    summary: pd.DataFrame, output: Path
) -> None:
    data = summary[summary.algorithm.isin(["prism_cc_time", "prism_cc_cost"])].copy()
    max_cost = max(data.budget_used_mean.max(), 1e-9)
    color_min = data.interference_per_task.min()
    color_max = data.interference_per_task.max()
    fig, ax = plt.subplots(figsize=(11, 7))
    for algorithm, marker in [("prism_cc_time", "o"), ("prism_cc_cost", "s")]:
        subset = (
            data[data.algorithm == algorithm]
            .set_index("scenario_id").reindex(SCENARIO_ORDER).reset_index()
        )
        subset_sizes = 100 + 500 * subset.budget_used_mean / max_cost
        scatter = ax.scatter(
            subset.makespan_mean, subset.makespan_cv_percent,
            s=subset_sizes, c=subset.interference_per_task,
            cmap="plasma", marker=marker, alpha=0.82,
            vmin=color_min, vmax=color_max,
            edgecolor="black", linewidth=0.6, label=ALGORITHM_LABELS[algorithm],
        )
        for number, row in enumerate(subset.itertuples(), start=1):
            ax.annotate(
                str(number),
                (row.makespan_mean, row.makespan_cv_percent),
                ha="center", va="center", fontsize=8, fontweight="bold",
            )
    colorbar = fig.colorbar(scatter, ax=ax)
    colorbar.set_label("Interferência média por atividade (s)")
    ax.set(
        title="Risco × desempenho — tamanho da bolha = custo médio",
        xlabel="Makespan médio (s)",
        ylabel="Coeficiente de variação do makespan (%)",
    )
    algorithm_handles, algorithm_labels = ax.get_legend_handles_labels()
    scenario_handles = [
        Line2D(
            [], [], linestyle="none", marker=f"${number}$", color="#222222",
            markersize=9,
            label=SCENARIO_LABELS[scenario].replace("\n", " "),
        )
        for number, scenario in enumerate(SCENARIO_ORDER, start=1)
    ]
    algorithm_legend = ax.legend(
        algorithm_handles, algorithm_labels, title="Algoritmo", loc="upper right"
    )
    ax.add_artist(algorithm_legend)
    ax.legend(
        handles=scenario_handles, title="Ambientes", ncol=2,
        loc="center right", frameon=True,
    )
    save(fig, output, "22-risco-versus-desempenho.png")


def recommendation_candidates(
    df: pd.DataFrame, manifest: dict
) -> pd.DataFrame:
    """Build robust recommendation options without exposing individual seeds."""
    rows = []
    for (scenario, algorithm), group in df.groupby(
        ["scenario_id", "algorithm"], observed=True
    ):
        makespan_mean = group.makespan.mean()
        cost_mean = group.budget_used.mean()
        makespan_p95 = group.makespan.quantile(0.95)
        cost_p95 = group.budget_used.quantile(0.95)
        rows.append(
            {
                "scenario_id": scenario,
                "algorithm": algorithm,
                "recommendation": (
                    f"{SCENARIO_LABELS[scenario].replace(chr(10), ' ')} · "
                    f"{ALGORITHM_LABELS[algorithm]}"
                ),
                "makespan_mean": makespan_mean,
                "makespan_p95": makespan_p95,
                "cost_mean": cost_mean,
                "cost_p95": cost_p95,
                "makespan_cv_percent": (
                    100 * group.makespan.std(ddof=1) / max(makespan_mean, 1e-9)
                ),
                "strict_feasibility_percent": 100 * group.feasible.mean(),
                "required_deadline_margin_percent": max(
                    0, 100 * (makespan_p95 / manifest["deadline_limit"] - 1)
                ),
                "required_budget_margin_percent": max(
                    0, 100 * (cost_p95 / manifest["budget_limit"] - 1)
                ),
            }
        )
    candidates = pd.DataFrame(rows)
    candidates["robust_feasible"] = (
        (candidates.cost_p95 <= manifest["budget_limit"])
        & (candidates.makespan_p95 <= manifest["deadline_limit"])
    )
    candidates["pareto"] = False
    representatives = (
        candidates.assign(
            cost_key=candidates.cost_p95.round(6),
            makespan_key=candidates.makespan_p95.round(6),
        )
        .sort_values(
            ["strict_feasibility_percent", "makespan_cv_percent"],
            ascending=[False, True],
        )
        .drop_duplicates(["cost_key", "makespan_key"])
    )
    for index, row in representatives.iterrows():
        dominated = (
            (representatives.cost_p95 <= row.cost_p95 + 1e-6)
            & (representatives.makespan_p95 <= row.makespan_p95 + 1e-6)
            & (
                (representatives.cost_p95 < row.cost_p95 - 1e-6)
                | (representatives.makespan_p95 < row.makespan_p95 - 1e-6)
            )
        ).any()
        candidates.loc[index, "pareto"] = not dominated
    frontier = candidates[candidates.pareto].sort_values(
        ["cost_p95", "makespan_p95"], ascending=[True, False]
    )
    candidates["frontier_order"] = pd.NA
    candidates["extra_cost_from_previous"] = np.nan
    candidates["time_saved_from_previous"] = np.nan
    candidates["seconds_saved_per_extra_dollar"] = np.nan
    previous = None
    for order, (index, row) in enumerate(frontier.iterrows(), start=1):
        candidates.loc[index, "frontier_order"] = order
        if previous is not None:
            extra_cost = row.cost_p95 - previous.cost_p95
            time_saved = previous.makespan_p95 - row.makespan_p95
            candidates.loc[index, "extra_cost_from_previous"] = extra_cost
            candidates.loc[index, "time_saved_from_previous"] = time_saved
            if extra_cost > 1e-9:
                candidates.loc[index, "seconds_saved_per_extra_dollar"] = (
                    time_saved / extra_cost
                )
        previous = row
    return candidates.sort_values(
        ["pareto", "frontier_order", "cost_p95"],
        ascending=[False, True, True],
    )


def plot_23_recommendation_frontier(
    candidates: pd.DataFrame, manifest: dict, output: Path
) -> None:
    """Show robust options and the marginal value of each concession."""
    frontier = candidates[candidates.pareto].sort_values("frontier_order")
    dominated = candidates[~candidates.pareto]
    fig, axes = plt.subplots(1, 2, figsize=(16, 7))

    ax = axes[0]
    ax.scatter(
        dominated.cost_p95, dominated.makespan_p95,
        s=45, color="#B7B7B7", marker="x", alpha=0.65,
        label="Opção dominada",
    )
    sizes = 90 + 5 * frontier.strict_feasibility_percent
    points = ax.scatter(
        frontier.cost_p95, frontier.makespan_p95,
        s=sizes, c=frontier.makespan_cv_percent, cmap="viridis",
        edgecolor="#222222", linewidth=0.7, zorder=3,
    )
    ax.plot(
        frontier.cost_p95, frontier.makespan_p95,
        color="#555555", linewidth=1.2, zorder=2,
    )
    for row in frontier.itertuples():
        ax.annotate(
            str(int(row.frontier_order)), (row.cost_p95, row.makespan_p95),
            ha="center", va="center", fontsize=8, fontweight="bold",
        )
    ax.axvline(
        manifest["budget_limit"], color="#B33A3A", linestyle="--", linewidth=1
    )
    ax.axhline(
        manifest["deadline_limit"], color="#B33A3A", linestyle="--", linewidth=1
    )
    ax.set(
        title="Opções robustas e fronteira de recomendação",
        xlabel="Custo P95 (USD)",
        ylabel="Makespan P95 (s)",
    )
    ax.legend(loc="upper right")
    colorbar = fig.colorbar(points, ax=ax, shrink=0.82)
    colorbar.set_label("Variabilidade do makespan — CV (%)")

    ax = axes[1]
    x = 100 * frontier.cost_p95 / manifest["budget_limit"]
    y = 100 * frontier.makespan_p95 / manifest["deadline_limit"]
    ax.plot(x, y, color="#2878B5", linewidth=2, marker="o", markersize=8)
    ax.fill_between([0, 100], 0, 100, color="#59A14F", alpha=0.10)
    for position, row in enumerate(frontier.itertuples()):
        ax.annotate(
            str(int(row.frontier_order)), (x.iloc[position], y.iloc[position]),
            ha="center", va="center", fontsize=8, fontweight="bold",
        )
        if position:
            label = (
                f"+US$ {row.extra_cost_from_previous:.2f}\n"
                f"−{row.time_saved_from_previous:.0f} s"
            )
            midpoint = (
                (x.iloc[position - 1] + x.iloc[position]) / 2,
                (y.iloc[position - 1] + y.iloc[position]) / 2,
            )
            ax.annotate(
                label, midpoint, xytext=(4, 5), textcoords="offset points",
                fontsize=8, ha="left",
            )
    ax.axvline(100, color="#B33A3A", linestyle="--", linewidth=1)
    ax.axhline(100, color="#B33A3A", linestyle="--", linewidth=1)
    ax.set(
        title="Concessões marginais: quanto abrir para quanto ganhar",
        xlabel="Uso do budget global (%)",
        ylabel="Uso do deadline global (%)",
    )

    recommendation_handles = [
        Line2D(
            [], [], linestyle="none", marker=f"${int(row.frontier_order)}$",
            color="#222222", markersize=9, label=row.recommendation,
        )
        for row in frontier.itertuples()
    ]
    fig.legend(
        handles=recommendation_handles, title="Recomendações na fronteira",
        loc="lower center", ncol=2, bbox_to_anchor=(0.5, -0.12),
        frameon=False,
    )
    fig.suptitle(
        "Sistema de recomendação — pequenas concessões e ganhos marginais",
        y=1.02,
    )
    fig.subplots_adjust(bottom=0.24, wspace=0.28)
    fig.savefig(
        output / "23-fronteira-recomendacoes-concessoes.png",
        dpi=180, bbox_inches="tight",
    )
    plt.close(fig)


def plot_24_feasible_options_by_environment(
    candidates: pd.DataFrame, manifest: dict, output: Path
) -> None:
    """Facet recommendation options by environment and delimit the HEFT SLA."""
    marker_by_algorithm = {
        "heft_classic": "D",
        "prism_cc_time": "o",
        "prism_cc_cost": "s",
    }
    short_label = {
        "heft_classic": "HEFT",
        "prism_cc_time": "Time",
        "prism_cc_cost": "Cost",
    }
    fig, axes = plt.subplots(2, 3, figsize=(16, 10))
    for ax, scenario in zip(axes.flat, SCENARIO_ORDER):
        data = (
            candidates[candidates.scenario_id == scenario]
            .set_index("algorithm").reindex(ALGORITHM_ORDER).reset_index()
        )
        x_max = max(
            manifest["budget_limit"] * 1.18,
            data.cost_p95.max() * 1.12,
            0.05,
        )
        y_max = max(
            manifest["deadline_limit"] * 1.18,
            data.makespan_p95.max() * 1.12,
        )
        ax.fill_between(
            [0, manifest["budget_limit"]],
            [0, 0],
            [manifest["deadline_limit"], manifest["deadline_limit"]],
            color="#59A14F", alpha=0.12, label="Região viável",
        )
        for row in data.itertuples():
            viable = bool(row.robust_feasible)
            label_offset = {
                "heft_classic": (7, 7),
                "prism_cc_time": (7, -25),
                "prism_cc_cost": (7, 7),
            }[row.algorithm]
            ax.scatter(
                row.cost_p95, row.makespan_p95,
                s=150, marker=marker_by_algorithm[row.algorithm],
                color=PALETTE[row.algorithm],
                alpha=0.92 if viable else 0.42,
                edgecolor="#1F1F1F" if viable else "#B33A3A",
                linewidth=1.1, zorder=4,
            )
            if not viable:
                ax.scatter(
                    row.cost_p95, row.makespan_p95,
                    s=52, marker="x", color="#B33A3A",
                    linewidth=1.5, zorder=5,
                )
            ax.annotate(
                (
                    f"{short_label[row.algorithm]}\n"
                    f"US$ {row.cost_p95:.2f} · {row.makespan_p95:.1f}s"
                ),
                (row.cost_p95, row.makespan_p95),
                xytext=label_offset, textcoords="offset points", fontsize=8,
            )
        ax.axvline(
            manifest["budget_limit"], color="#B33A3A",
            linestyle="--", linewidth=1.2,
        )
        ax.axhline(
            manifest["deadline_limit"], color="#B33A3A",
            linestyle="--", linewidth=1.2,
        )
        ax.set_xlim(left=-0.025 * x_max, right=x_max)
        ax.set_ylim(bottom=-0.025 * y_max, top=y_max)
        ax.set_title(SCENARIO_LABELS[scenario].replace("\n", " "))
        ax.set_xlabel("Custo P95 (USD)")
        ax.set_ylabel("Makespan P95 (s)")
        viable_count = int(data.robust_feasible.sum())
        ax.text(
            0.98, 0.04, f"{viable_count}/3 opções viáveis",
            transform=ax.transAxes, ha="right", va="bottom",
            fontsize=8, color="#333333",
        )
    legend_handles = [
        Line2D(
            [], [], linestyle="none", marker=marker_by_algorithm[algorithm],
            markerfacecolor=PALETTE[algorithm], markeredgecolor="#1F1F1F",
            markersize=9, label=ALGORITHM_LABELS[algorithm],
        )
        for algorithm in ALGORITHM_ORDER
    ]
    legend_handles.extend(
        [
            Patch(facecolor="#59A14F", alpha=0.18, label="Dentro do budget e deadline"),
            Line2D(
                [], [], linestyle="none", marker="x", color="#B33A3A",
                markersize=8, label="Fora do SLA robusto",
            ),
        ]
    )
    fig.legend(
        handles=legend_handles, loc="lower center", ncol=5,
        bbox_to_anchor=(0.5, -0.01), frameon=False,
    )
    fig.suptitle(
        "Opções viáveis por ambiente — limites globais derivados do HEFT",
        y=1.01,
    )
    fig.subplots_adjust(bottom=0.09, hspace=0.32, wspace=0.22)
    fig.savefig(
        output / "24-opcoes-viaveis-por-ambiente.png",
        dpi=180, bbox_inches="tight",
    )
    plt.close(fig)


def plot_13_priority_comparison(repo_root: Path, df: pd.DataFrame, output: Path) -> bool:
    if EXPERIMENT_RESULT_DIR != "prism-cc-uprank-order-exp-01":
        return False
    topology_path = (
        repo_root / "experiments" / "results"
        / "prism-cc-topology-order-exp-01" / "raw_results.csv"
    )
    if not topology_path.exists():
        return False
    topology = pd.read_csv(topology_path)
    algorithms = ["prism_cc_time", "prism_cc_cost"]
    topology = topology[topology["algorithm"].isin(algorithms)].copy()
    uprank = df[df["algorithm"].isin(algorithms)].copy()
    topology["priority"] = "Topology"
    uprank["priority"] = "UpRank"
    combined = pd.concat([topology, uprank], ignore_index=True)
    combined["series"] = combined["algorithm"].map(ALGORITHM_LABELS) + " / " + combined["priority"]

    series_order = [
        "PRISM-CC - Time / Topology",
        "PRISM-CC - Time / UpRank",
        "PRISM-CC - Cost / Topology",
        "PRISM-CC - Cost / UpRank",
    ]
    colors = {
        series_order[0]: "#8AB8D8",
        series_order[1]: "#2878B5",
        series_order[2]: "#9BC493",
        series_order[3]: "#3F7F36",
    }
    fig, axes = plt.subplots(2, 1, figsize=(13, 11))
    for ax, metric, ylabel in [
        (axes[0], "makespan", "Makespan (s)"),
        (axes[1], "budget_used", "Custo total (USD)"),
    ]:
        sns.boxplot(
            data=combined, x="scenario_id", y=metric, hue="series",
            order=SCENARIO_ORDER, hue_order=series_order, palette=colors, ax=ax,
        )
        ax.set(xlabel="", ylabel=ylabel)
        ax.set_xticklabels([SCENARIO_LABELS[item] for item in SCENARIO_ORDER])
        ax.legend(title="", ncol=2)
    axes[0].set_title("Efeito da prioridade no makespan")
    axes[1].set_title("Efeito da prioridade no custo")
    fig.suptitle("PRISM-CC: ordem topológica × upward rank", y=1.01, fontweight="bold")
    save(fig, output, "13-comparacao-topology-uprank.png")

    pivot = combined.pivot(
        index=["scenario_id", "interference_seed", "algorithm"],
        columns="priority", values=["makespan", "budget_used"],
    )
    comparison = pd.DataFrame({
        "topology_makespan": pivot[("makespan", "Topology")],
        "uprank_makespan": pivot[("makespan", "UpRank")],
        "topology_cost": pivot[("budget_used", "Topology")],
        "uprank_cost": pivot[("budget_used", "UpRank")],
    }).reset_index()
    comparison["makespan_gain_uprank_percent"] = (
        100 * (comparison["topology_makespan"] - comparison["uprank_makespan"])
        / comparison["topology_makespan"].clip(lower=1e-9)
    )
    comparison["cost_gain_uprank_percent"] = (
        100 * (comparison["topology_cost"] - comparison["uprank_cost"])
        / comparison["topology_cost"].clip(lower=1e-9)
    )
    comparison.to_csv(output.parent / "priority_comparison.csv", index=False)
    return True


def write_report(output: Path, manifest: dict, has_priority_comparison: bool = False) -> None:
    priority_label = {
        "topological_order": "ordem topológica",
        "upward_rank": "upward rank",
    }.get(manifest.get("prism_cc_priority"), manifest.get("prism_cc_priority", "não informada"))
    descriptions = [
        ("01-makespan-por-ambiente.png", "Makespan por ambiente e algoritmo", "Boxplots das 30 sementes do PRISM-CC. A linha vermelha é o deadline global, calculado como a média das execuções HEFT clássico. Como o HEFT clássico não possui co-alocação nem interferência, seu resultado é determinístico e aparece como uma linha em cada ambiente."),
        ("02-custo-por-ambiente.png", "Custo por ambiente e algoritmo", "Compara o custo total das 30 execuções. A linha vermelha é o budget global, calculado como a média de todos os custos HEFT. Cenários on-premise aparecem com custo financeiro zero."),
        ("03-factibilidade.png", "Factibilidade conjunta", "Percentual de execuções que respeitaram simultaneamente budget e deadline. É a leitura mais direta de cumprimento do SLA."),
        ("04-custo-versus-makespan.png", "Trade-off custo × makespan", "Cada ponto é uma execução. As linhas vermelhas representam o budget e o deadline globais definidos pelas médias de todas as execuções HEFT."),
        ("05-ganho-prism-cc-sobre-heft.png", "Ganho pareado das variantes PRISM-CC sobre HEFT", "Diferenças calculadas semente a semente para PRISM-CC Time e PRISM-CC Cost. Valores positivos favorecem a variante PRISM-CC; negativos favorecem o HEFT clássico. O painel esquerdo mede makespan e o direito mede custo."),
        ("06-interferencia-versus-makespan.png", "Interferência × makespan", "Relaciona o tempo total adicionado pela interferência ao makespan, com tendência linear por algoritmo. Indica quanto o atraso de interferência chega ao caminho crítico."),
        ("07-pares-versus-makespan.png", "Pares interferentes × makespan", "Relaciona quantos pares realmente se sobrepuseram na mesma máquina ao makespan. Distingue atividades selecionadas de interferências efetivamente ativadas."),
        ("08-tempo-de-interferencia.png", "Distribuição do tempo de interferência", "Boxplots do overhead total provocado pela interferência. Permite comparar a capacidade de cada escalonador de evitar sobreposições prejudiciais."),
        ("09-heatmap-utilizacao.png", "Heatmap de utilização", "Utilização média de cada máquina considerando seus cores e o makespan. Células escuras indicam maior ocupação relativa."),
        ("10-distribuicao-atividades.png", "Distribuição das atividades", f"Barras empilhadas com a parcela média das {manifest.get('task_count', 58)} atividades destinada às famílias Bora, Diablo, H3 e H4D."),
        ("11-tempo-computacional.png", "Tempo dos algoritmos", "Custo computacional do escalonamento em escala logarítmica. Evidencia a diferença de tempo entre a busca Beam e o HEFT."),
        ("12-estabilidade-por-semente.png", "Estabilidade entre sementes", "Acompanha o makespan nas 30 seleções pareadas de atividades interferentes. Oscilações mostram sensibilidade à composição da interferência."),
        ("14-makespan-agregado-ic95.png", "Makespan agregado por ambiente", "Cada painel apresenta um algoritmo. O círculo é a média das repetições, a barra horizontal é o intervalo de confiança de 95% e o losango vazado é a mediana. As sementes não são exibidas individualmente; elas servem para estimar a incerteza."),
        ("15-custo-agregado-ic95.png", "Custo agregado por ambiente", "Resume o custo das repetições com média, intervalo de confiança de 95% e mediana. A proximidade entre média e mediana indica estabilidade; diferenças maiores sugerem assimetria ou execuções atípicas."),
        ("16-relacao-agregada-custo-makespan.png", "Relação agregada entre custo e makespan", "Cada ambiente contém somente um ponto por algoritmo. A posição representa as médias de custo e makespan e as barras mostram os respectivos intervalos de confiança de 95%."),
        ("17-placar-prism-cc-versus-heft.png", "Placar PRISM-CC versus HEFT", "Resume quem apresentou menor makespan e menor custo usando as médias das 30 execuções. Verde indica vitória do PRISM-CC, vermelho vitória do HEFT e células neutras indicam empate. O percentual quantifica a redução relativa em relação ao HEFT."),
        ("18-barras-e-linhas-prism-cc-versus-heft.png", "Barras e linhas PRISM-CC versus HEFT", "As barras mostram as médias reais de makespan e custo, com intervalo de confiança de 95%. As linhas usam o segundo eixo para mostrar a redução percentual de PRISM-CC Time e Cost em relação ao HEFT. O eixo de makespan é logarítmico para manter visíveis algoritmos com ordens de grandeza diferentes."),
        ("19-mapa-multidimensional-ganhos.png", "Mapa multidimensional de ganhos", "Combina redução de custo, redução de makespan, interferência média por atividade e desequilíbrio de utilização. Pontos no quadrante superior direito favorecem o PRISM-CC nas duas métricas."),
        ("20-preco-da-economia.png", "Preço da economia", "Mostra quantos segundos são adicionados ao makespan e quantos dólares são economizados ao escolher PRISM-CC Cost no lugar de PRISM-CC Time. As barras de erro representam IC 95% e os rótulos apresentam segundos adicionais por dólar."),
        ("21-alocacao-explica-resultado.png", "Alocação explicando o resultado", "As barras empilhadas mostram a parcela média das atividades atribuída a cada família de máquinas. A linha mostra o makespan médio, o tamanho dos marcadores representa o custo e os rótulos apresentam ambos os valores."),
        ("22-risco-versus-desempenho.png", "Risco versus desempenho", "Relaciona makespan médio e coeficiente de variação. O tamanho da bolha representa o custo médio e a cor representa a interferência média por atividade."),
        ("23-fronteira-recomendacoes-concessoes.png", "Fronteira de recomendações e concessões", "Apresenta somente as recomendações não dominadas usando custo e makespan no percentil 95, evitando decidir apenas pela média. O primeiro painel mostra a fronteira robusta, a factibilidade e a variabilidade. O segundo transforma a fronteira em uma sequência de concessões: quanto custo adicional é necessário aceitar e quantos segundos são economizados ao migrar para a próxima recomendação."),
        ("24-opcoes-viaveis-por-ambiente.png", "Opções viáveis por ambiente", "Cada painel representa um ambiente e cada ponto uma opção de escalonamento: HEFT clássico, PRISM-CC Time ou PRISM-CC Cost. A área verde é delimitada pelo budget e pelo deadline globais derivados do HEFT. Para considerar a incerteza das repetições, custo e makespan são apresentados no percentil 95; pontos com uma marca vermelha ficaram fora de pelo menos um dos limites."),
    ]
    if has_priority_comparison:
        descriptions.append(
            (
                "13-comparacao-topology-uprank.png",
                "Comparação Topology × UpRank",
                "Compara diretamente as duas políticas de prioridade do PRISM-CC, mantendo máquinas, sementes, interferência, Beam Search, budget e deadline constantes. O painel superior mostra makespan e o inferior mostra custo.",
            )
        )
    lines = [
        "# Gráficos do protocolo experimental",
        "",
        f"Prioridade das tarefas do PRISM-CC: **{priority_label}**.",
        "",
        f"Budget global (média HEFT): `{manifest['budget_limit']}`. "
        f"Deadline global (média HEFT): `{manifest['deadline_limit']}` segundos. "
        f"Cada combinação possui {len(manifest['interference_seeds'])} sementes pareadas.",
        "",
    ]
    for filename, title, description in descriptions:
        lines.extend([f"## {title}", "", description, "", f"![{title}]({filename})", ""])
    (output / "README.md").write_text("\n".join(lines), encoding="utf-8")


def generate_all(repo_root: Path) -> list[Path]:
    configure_style()
    df, manifest = load_results(repo_root)
    validate_results(df, manifest)
    output = repo_root / "experiments" / "results" / EXPERIMENT_RESULT_DIR / "figures"
    output.mkdir(parents=True, exist_ok=True)
    plot_01_makespan(df, manifest, output)
    plot_02_cost(df, manifest, output)
    plot_03_feasibility(df, output)
    plot_04_cost_makespan(df, manifest, output)
    plot_05_gain(df, output)
    faceted_scatter(df, "interference_time", "06-interferencia-versus-makespan.png", "Tempo de interferência × makespan", "Tempo de interferência (s)", output)
    faceted_scatter(df, "interference_pairs", "07-pares-versus-makespan.png", "Pares interferentes × makespan", "Pares interferentes", output)
    plot_08_interference_distribution(df, output)
    plot_09_utilization(df, output)
    plot_10_distribution(df, output)
    plot_11_algorithm_time(df, output)
    plot_12_seed_stability(df, output)
    has_priority_comparison = plot_13_priority_comparison(repo_root, df, output)
    aggregate = aggregate_environment_statistics(df)
    aggregate.to_csv(output.parent / "aggregate_environment_summary.csv", index=False)
    plot_14_aggregate_forest(
        aggregate, "makespan", "14-makespan-agregado-ic95.png",
        "Makespan por ambiente — média, IC 95% e mediana", "Makespan (s)", output,
    )
    plot_14_aggregate_forest(
        aggregate, "budget_used", "15-custo-agregado-ic95.png",
        "Custo por ambiente — média, IC 95% e mediana", "Custo total (USD)", output,
    )
    plot_16_aggregate_cost_makespan(aggregate, output)
    winner_summary = plot_17_winner_scorecard(aggregate, output)
    winner_summary.to_csv(output.parent / "winner_vs_heft_summary.csv", index=False)
    plot_18_bars_and_gain_lines(aggregate, output)
    multidimensional = multidimensional_summary(df, aggregate, manifest)
    multidimensional.to_csv(
        output.parent / "multidimensional_environment_summary.csv", index=False
    )
    plot_19_multidimensional_gain(multidimensional, output)
    tradeoff = plot_20_price_of_savings(df, output)
    tradeoff.to_csv(output.parent / "time_cost_tradeoff_summary.csv", index=False)
    plot_21_allocation_explains_result(df, aggregate, output)
    plot_22_risk_performance(multidimensional, output)
    recommendations = recommendation_candidates(df, manifest)
    recommendations.to_csv(
        output.parent / "recommendation_candidates.csv", index=False
    )
    recommendations[recommendations.pareto].to_csv(
        output.parent / "recommendation_frontier.csv", index=False
    )
    plot_23_recommendation_frontier(recommendations, manifest, output)
    plot_24_feasible_options_by_environment(recommendations, manifest, output)
    write_report(output, manifest, has_priority_comparison)
    return sorted(output.glob("*.png"))
