# Impacto da interferência de software

Experimento no ambiente híbrido Fog–HPC–Cloud com rede real com workflow de processamento de imagens · 8 tarefas · saídas de 10 GB,
1 sementes pareadas,
PRISM com **caminho HEFT canônico + Beam adaptativo**, HEFT com co-location e Beam 120.

A penalidade por tarefa interferente sobreposta varia de 10% a 90%.
O conjunto de atividades interferentes permanece pareado e constante.
O SLA foi calibrado sem interferência e mantido fixo em todas as execuções:

- Deadline: 51.573600 s
- Budget: US$ 0.009311

![Impacto da interferência](figures/impacto-interferencia.png)

## Gráficos individuais

- [Makespan por interferência](figures/makespan-por-interferencia.png)
- [Custo por interferência](figures/custo-por-interferencia.png)
- [Interferência acumulada](figures/interferencia-acumulada.png)
- [Factibilidade por interferência](figures/factibilidade-por-interferencia.png)
