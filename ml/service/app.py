import os

from fastapi import FastAPI, File, UploadFile
from fastapi.responses import JSONResponse

from ml.src.classifier import DEFAULT_CONFIDENCE_THRESHOLD, MAX_FILE_SIZE_BYTES, WasteClassifier

MODEL_PATH = os.environ.get("MODEL_PATH", "ml/models/mobilenetv3_waste_new.pth")
CONFIDENCE_THRESHOLD = float(os.environ.get("CONFIDENCE_THRESHOLD", DEFAULT_CONFIDENCE_THRESHOLD))

app = FastAPI(title="RaspberryBin ML service")

classifier = WasteClassifier(MODEL_PATH, CONFIDENCE_THRESHOLD)


@app.post("/predict")
def predict(image: UploadFile = File(...)):
    data = image.file.read(MAX_FILE_SIZE_BYTES + 1)
    result = classifier.predict_bytes(data)

    status_code = 200 if result["status"] == "ok" else 400
    return JSONResponse(result, status_code=status_code)
