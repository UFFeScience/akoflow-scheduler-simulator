from __future__ import annotations

import json
from pathlib import Path

import matplotlib

matplotlib.use("Agg")

import matplotlib.pyplot as plt
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
EXPERIMENT_RESULT_DIR = "prism-cc-topology-order-exp-01"


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
    assert df["interference_activity_ids"].str.split("|", regex=False).str.len().eq(29).all()
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


def write_report(output: Path, manifest: dict) -> None:
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
        ("10-distribuicao-atividades.png", "Distribuição das atividades", "Barras empilhadas com a parcela média das 58 atividades destinada às famílias Bora, Diablo, H3 e H4D."),
        ("11-tempo-computacional.png", "Tempo dos algoritmos", "Custo computacional do escalonamento em escala logarítmica. Evidencia a diferença de tempo entre a busca Beam e o HEFT."),
        ("12-estabilidade-por-semente.png", "Estabilidade entre sementes", "Acompanha o makespan nas 30 seleções pareadas de atividades interferentes. Oscilações mostram sensibilidade à composição da interferência."),
    ]
    lines = [
        "# Gráficos do protocolo experimental",
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
    write_report(output, manifest)
    return sorted(output.glob("*.png"))
