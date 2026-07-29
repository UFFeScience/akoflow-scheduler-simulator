from __future__ import annotations

import argparse
from pathlib import Path


COMPARISON_FIGURES = {
    "01-makespan-por-ambiente.png",
    "02-custo-por-ambiente.png",
    "03-factibilidade.png",
    "04-custo-versus-makespan.png",
    "05-ganho-prism-cc-sobre-heft.png",
    "06-interferencia-versus-makespan.png",
    "07-pares-versus-makespan.png",
    "08-tempo-de-interferencia.png",
    "09-heatmap-utilizacao.png",
    "10-distribuicao-atividades.png",
    "11-tempo-computacional.png",
    "12-estabilidade-por-semente.png",
    "13-violacoes-sla.png",
    "14-makespan-agregado-ic95.png",
    "15-custo-agregado-ic95.png",
    "16-relacao-agregada-custo-makespan.png",
    "17-placar-prism-cc-versus-heft.png",
    "18-barras-e-linhas-prism-cc-versus-heft.png",
    "19-mapa-multidimensional-ganhos.png",
    "20-preco-da-economia.png",
    "21-alocacao-explica-resultado.png",
    "22-risco-versus-desempenho.png",
    "23-fronteira-recomendacoes-concessoes.png",
    "24-opcoes-viaveis-por-ambiente.png",
    "25-lista-n-recomendacoes-por-ambiente.png",
    "26-resumo-executivo.png",
}

SWEEP_FIGURES = {
    "impacto-interferencia.png",
    "makespan-por-interferencia.png",
    "custo-por-interferencia.png",
    "interferencia-acumulada.png",
    "factibilidade-por-interferencia.png",
}


def validate(result_dir: Path, expected: set[str]) -> list[str]:
    figures = result_dir / "figures"
    actual = {path.name for path in figures.glob("*.png")}
    errors = []
    missing = sorted(expected - actual)
    extra = sorted(actual - expected)
    empty = sorted(path.name for path in figures.glob("*.png") if path.stat().st_size < 10_000)
    if missing:
        errors.append(f"missing={missing}")
    if extra:
        errors.append(f"extra={extra}")
    if empty:
        errors.append(f"small_or_empty={empty}")
    return errors


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("results_root", type=Path)
    args = parser.parse_args()
    failures = []
    checked = 0
    for result_dir in sorted(path for path in args.results_root.iterdir() if path.is_dir()):
        name = result_dir.name
        expected = None
        if "-vs-heft-" in name and (result_dir / "raw_results.csv").exists():
            expected = COMPARISON_FIGURES
        elif "interference-sweep" in name:
            rate_count = len(list(result_dir.glob("rate-*")))
            if rate_count == 9:
                expected = SWEEP_FIGURES
        if expected is None:
            continue
        checked += 1
        errors = validate(result_dir, expected)
        if errors:
            failures.append(f"{name}: {'; '.join(errors)}")
    if failures:
        raise SystemExit("\n".join(failures))
    print(f"validated {checked} result directories")


if __name__ == "__main__":
    main()
