FROM python:3.12-slim

WORKDIR /workspace

COPY experiments/notebooks/requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir -r /tmp/requirements.txt

CMD ["sh", "-lc", "jupyter nbconvert --to notebook --execute --inplace --ExecutePreprocessor.timeout=600 experiments/notebooks/01-validacao-dados.ipynb && jupyter nbconvert --to notebook --execute --inplace --ExecutePreprocessor.timeout=600 experiments/notebooks/02-graficos-experimento.ipynb"]
