# Gráficos do protocolo experimental

Prioridade das tarefas do PRISM-CC: **adaptive_ready**.

SLA definido por ambiente: budget com margem `0.9x` e deadline com margem `0.9x` sobre a média do HEFT usado como baseline. Cada combinação possui 1 sementes pareadas.

## Makespan por ambiente e algoritmo

Boxplots das 30 sementes do PRISM-CC. Cada segmento vermelho indica o deadline específico do ambiente, definido como 1,2× a média do HEFT usado como baseline naquele ambiente.

![Makespan por ambiente e algoritmo](01-makespan-por-ambiente.png)

## Custo por ambiente e algoritmo

Compara o custo total das 30 execuções. Cada segmento vermelho indica o budget específico do ambiente, definido como 1,2× a média do HEFT usado como baseline. Cenários on-premise aparecem com custo financeiro zero.

![Custo por ambiente e algoritmo](02-custo-por-ambiente.png)

## Factibilidade conjunta

Percentual de execuções que respeitaram simultaneamente budget e deadline. É a leitura mais direta de cumprimento do SLA.

![Factibilidade conjunta](03-factibilidade.png)

## Trade-off custo × makespan

Cada ponto é uma execução. Em cada painel, as linhas vermelhas representam o budget e o deadline específicos daquele ambiente.

![Trade-off custo × makespan](04-custo-versus-makespan.png)

## Ganho pareado das variantes PRISM-CC sobre HEFT

Diferenças calculadas semente a semente para PRISM-CC Time e PRISM-CC Cost. Valores positivos favorecem a variante PRISM-CC; negativos favorecem o HEFT clássico. O painel esquerdo mede makespan e o direito mede custo.

![Ganho pareado das variantes PRISM-CC sobre HEFT](05-ganho-prism-cc-sobre-heft.png)

## Interferência × makespan

Relaciona o tempo total adicionado pela interferência ao makespan, com tendência linear por algoritmo. Indica quanto o atraso de interferência chega ao caminho crítico.

![Interferência × makespan](06-interferencia-versus-makespan.png)

## Pares interferentes × makespan

Relaciona quantos pares realmente se sobrepuseram na mesma máquina ao makespan. Distingue atividades selecionadas de interferências efetivamente ativadas.

![Pares interferentes × makespan](07-pares-versus-makespan.png)

## Distribuição do tempo de interferência

Boxplots do overhead total provocado pela interferência. Permite comparar a capacidade de cada escalonador de evitar sobreposições prejudiciais.

![Distribuição do tempo de interferência](08-tempo-de-interferencia.png)

## Heatmap de utilização

Utilização média de cada máquina considerando seus cores e o makespan. Células escuras indicam maior ocupação relativa.

![Heatmap de utilização](09-heatmap-utilizacao.png)

## Distribuição das atividades

Barras empilhadas com a parcela média das 6448 atividades destinada às famílias Bora, Diablo, H3 e H4D.

![Distribuição das atividades](10-distribuicao-atividades.png)

## Tempo dos algoritmos

Custo computacional do escalonamento em escala logarítmica. Evidencia a diferença de tempo entre a busca Beam e o HEFT.

![Tempo dos algoritmos](11-tempo-computacional.png)

## Estabilidade entre sementes

Acompanha o makespan nas 30 seleções pareadas de atividades interferentes. Oscilações mostram sensibilidade à composição da interferência.

![Estabilidade entre sementes](12-estabilidade-por-semente.png)

## Makespan agregado por ambiente

Cada painel apresenta um algoritmo. O círculo é a média das repetições, a barra horizontal é o intervalo de confiança de 95% e o losango vazado é a mediana. As sementes não são exibidas individualmente; elas servem para estimar a incerteza.

![Makespan agregado por ambiente](14-makespan-agregado-ic95.png)

## Custo agregado por ambiente

Resume o custo das repetições com média, intervalo de confiança de 95% e mediana. A proximidade entre média e mediana indica estabilidade; diferenças maiores sugerem assimetria ou execuções atípicas.

![Custo agregado por ambiente](15-custo-agregado-ic95.png)

## Relação agregada entre custo e makespan

Cada ambiente contém somente um ponto por algoritmo. A posição representa as médias de custo e makespan e as barras mostram os respectivos intervalos de confiança de 95%.

![Relação agregada entre custo e makespan](16-relacao-agregada-custo-makespan.png)

## Placar PRISM-CC versus HEFT

Resume quem apresentou menor makespan e menor custo usando as médias das 30 execuções. Verde indica vitória do PRISM-CC, vermelho vitória do HEFT e células neutras indicam empate. O percentual quantifica a redução relativa em relação ao HEFT.

![Placar PRISM-CC versus HEFT](17-placar-prism-cc-versus-heft.png)

## Barras e linhas PRISM-CC versus HEFT

As barras mostram as médias reais de makespan e custo, com intervalo de confiança de 95%. As linhas usam o segundo eixo para mostrar a redução percentual de PRISM-CC Time e Cost em relação ao HEFT. O eixo de makespan é logarítmico para manter visíveis algoritmos com ordens de grandeza diferentes.

![Barras e linhas PRISM-CC versus HEFT](18-barras-e-linhas-prism-cc-versus-heft.png)

## Mapa multidimensional de ganhos

Combina redução de custo, redução de makespan, interferência média por atividade e desequilíbrio de utilização. Pontos no quadrante superior direito favorecem o PRISM-CC nas duas métricas.

![Mapa multidimensional de ganhos](19-mapa-multidimensional-ganhos.png)

## Preço da economia

Mostra quantos segundos são adicionados ao makespan e quantos dólares são economizados ao escolher PRISM-CC Cost no lugar de PRISM-CC Time. As barras de erro representam IC 95% e os rótulos apresentam segundos adicionais por dólar.

![Preço da economia](20-preco-da-economia.png)

## Alocação explicando o resultado

As barras empilhadas mostram a parcela média das atividades atribuída a cada família de máquinas. A linha mostra o makespan médio, o tamanho dos marcadores representa o custo e os rótulos apresentam ambos os valores.

![Alocação explicando o resultado](21-alocacao-explica-resultado.png)

## Risco versus desempenho

Relaciona makespan médio e coeficiente de variação. O tamanho da bolha representa o custo médio e a cor representa a interferência média por atividade.

![Risco versus desempenho](22-risco-versus-desempenho.png)

## Fronteira de recomendações e concessões

Apresenta somente as recomendações não dominadas usando custo e makespan no percentil 95, evitando decidir apenas pela média. O primeiro painel mostra a fronteira robusta, a factibilidade e a variabilidade. O segundo transforma a fronteira em uma sequência de concessões: quanto custo adicional é necessário aceitar e quantos segundos são economizados ao migrar para a próxima recomendação.

![Fronteira de recomendações e concessões](23-fronteira-recomendacoes-concessoes.png)

## Opções viáveis por ambiente

Cada painel representa um ambiente e cada ponto uma opção de escalonamento: HEFT, PRISM-CC Time ou PRISM-CC Cost. A área verde é delimitada pelo budget e pelo deadline específicos daquele ambiente. Para considerar a incerteza das repetições, custo e makespan são apresentados no percentil 95; pontos com uma marca vermelha ficaram fora de pelo menos um dos limites.

![Opções viáveis por ambiente](24-opcoes-viaveis-por-ambiente.png)

## Resumo executivo relativo ao HEFT

Normaliza makespan, custo e tempo computacional pela média do HEFT usado como baseline. Valores abaixo de 1 favorecem o algoritmo; valores acima de 1 representam aumento relativo.

![Resumo executivo relativo ao HEFT](26-resumo-executivo.png)

## Violações do SLA

Mostra, separadamente, o percentual de execuções que violou o deadline e o budget em cada ambiente e algoritmo.

![Violações do SLA](13-violacoes-sla.png)

## Lista N de recomendações por ambiente

Exibe todas as recomendações exportadas pelo Beam para PRISM-CC Time e PRISM-CC Cost. Recomendações que excederam budget ou deadline aparecem em cinza ao fundo. Se o resultado não tiver exportado `recommendations_json`, o próprio gráfico registra explicitamente que a lista N está indisponível.

![Lista N de recomendações por ambiente](25-lista-n-recomendacoes-por-ambiente.png)
