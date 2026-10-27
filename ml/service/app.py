import os

from fastapi import FastAPI, File, UploadFile
from fastapi.responses import JSONResponse

from ml.src.classifier import MAX_FILE_SIZE_BYTES, WasteClassifier

MODEL_PATH = os.environ.get("MODEL_PATH", "ml/models/model_ConvNeXt-Small.pth")

# Optional override; by default the threshold stored in the model file is used
CONFIDENCE_THRESHOLD = os.environ.get("CONFIDENCE_THRESHOLD")

# No interactive API docs: the service is internal and only called by the backend
app = FastAPI(title="RaspberryBin ML service", docs_url=None, redoc_url=None, openapi_url=None)

classifier = WasteClassifier(MODEL_PATH, float(CONFIDENCE_THRESHOLD) if CONFIDENCE_THRESHOLD else None)


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/predict")
def predict(image: UploadFile = File(...)):
    data = image.file.read(MAX_FILE_SIZE_BYTES + 1)
    result = classifier.predict_bytes(data)

    status_code = 200 if result["status"] == "ok" else 400
    return JSONResponse(result, status_code=status_code)
