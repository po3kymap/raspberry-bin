# Built from the repository root (see backend/docker-compose.yml), because the
# code imports the "ml" package: ml.src.classifier, ml.service.app.
FROM python:3.13-slim

WORKDIR /app

COPY ml/requirements.txt ml/requirements.txt
RUN pip install --no-cache-dir --index-url https://download.pytorch.org/whl/cpu torch==2.14.1 torchvision==0.29.1 && \
    pip install --no-cache-dir -r ml/requirements.txt

COPY ml/src ml/src
COPY ml/service ml/service
COPY ml/tests ml/tests
COPY ml/models/model_ConvNeXt-Small.pth ml/models/

# Do not run as root inside the container
RUN useradd --create-home --uid 10001 mluser
USER mluser

ENV MODEL_PATH=ml/models/model_ConvNeXt-Small.pth \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

EXPOSE 5000

# One worker: each worker loads its own copy of the model into memory
CMD ["uvicorn", "ml.service.app:app", "--host", "0.0.0.0", "--port", "5000", "--workers", "1"]
