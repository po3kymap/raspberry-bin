import io
import os

import torch
from PIL import Image, ImageOps, UnidentifiedImageError
from ml.src.train import build_model, eval_transform

MAX_FILE_SIZE_BYTES = 10 * 1024 * 1024
MAX_IMAGE_PIXELS = 4096 * 4096
ALLOWED_FORMATS = {"JPEG", "PNG"}

SORTABLE_CATEGORIES = {"GENERAL_WASTE", "ORGANIC", "PAPER", "PMD"}


def choose_device():
    if torch.cuda.is_available():
        return torch.device("cuda")
    if torch.backends.mps.is_available():
        return torch.device("mps")
    return torch.device("cpu")


def load_model(model_path, device):
    """Load a model saved by train.py. The file stores everything needed to rebuild it:
    architecture, categories, image size and the confidence threshold chosen on the validation set."""
    checkpoint = torch.load(model_path, map_location="cpu", weights_only=True)
    categories = checkpoint["categories"]
    image_size = checkpoint.get("image_size", 224)

    model = build_model(checkpoint["arch"], len(categories), pretrained=False, image_size=image_size)
    model.load_state_dict(checkpoint["state_dict"])
    model.eval()

    return model.to(device), categories, image_size, checkpoint.get("threshold", 0.5)


def error_response(reason, message):
    return {
        "status": "error",
        "reason": reason,
        "message": message,
        "can_sort": False,
    }


class WasteClassifier:
    def __init__(self, model_path, confidence_threshold=None):
        self.model_path = model_path
        self.device = choose_device()
        self.model, self.categories, image_size, saved_threshold = load_model(self.model_path, self.device)
        self.preprocess = eval_transform(image_size)

        # Use the threshold stored with the model unless one is given explicitly
        self.confidence_threshold = saved_threshold if confidence_threshold is None else confidence_threshold


    def predict(self, image_path):
        try:
            if os.path.getsize(image_path) > MAX_FILE_SIZE_BYTES:
                return error_response("file_too_large", "Image file is too large.")

            with open(image_path, "rb") as file:
                data = file.read()
        except FileNotFoundError:
            return error_response("file_not_found", "Image file not found.")

        except OSError:
            return error_response("unreadable_image", "Image could not be read.")

        return self.predict_bytes(data)


    def predict_bytes(self, data):
        if len(data) > MAX_FILE_SIZE_BYTES:
            return error_response("file_too_large", "Image file is too large.")

        try:
            with Image.open(io.BytesIO(data)) as image:
                if image.format not in ALLOWED_FORMATS:
                    return error_response("unsupported_format", "Only JPEG and PNG images are accepted.")

                width, height = image.size
                if width * height > MAX_IMAGE_PIXELS:
                    return error_response("image_too_large", "Image resolution is too large.")

                # Same as training: apply the camera's EXIF rotation, then convert to RGB
                image = ImageOps.exif_transpose(image).convert("RGB")
                tensor = self.preprocess(image).unsqueeze(0)
        except Image.DecompressionBombError:
            return error_response("image_too_large", "Image resolution is too large.")

        except (UnidentifiedImageError, OSError):
            return error_response("unreadable_image", "Image could not be read.")

        tensor = tensor.to(self.device)

        with torch.inference_mode():
            scores = self.model(tensor)
            probabilities = torch.softmax(scores, dim=1)[0]

        index = probabilities.argmax().item()
        category = self.categories[index]
        confidence = probabilities[index].item()

        if category not in SORTABLE_CATEGORIES:
            can_sort, reason = False, "unsupported_item"
        elif confidence < self.confidence_threshold:
            can_sort, reason = False, "low_confidence"
        else:
            can_sort, reason = True, None

        return {
            "status": "ok",
            "category": category,
            "confidence": confidence,
            "can_sort": can_sort,
            "reason": reason,
        }



if __name__ == "__main__":
    classifier = WasteClassifier("ml/models/model_ConvNeXt-Small.pth")
    print("Device:", classifier.device)
    print("Categories: ", classifier.categories)
    result = classifier.predict("ml/data/example.jpg")
    print(result)
