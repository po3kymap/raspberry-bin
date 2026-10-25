import io

import pytest
from PIL import Image

from ml.src.classifier import MAX_FILE_SIZE_BYTES, WasteClassifier

MODEL_PATH = "ml/models/mobilenetv3_waste_new.pth"
CATEGORIES = {"GENERAL_WASTE", "ORGANIC", "PAPER", "PMD", "UNSUPPORTED"}


@pytest.fixture(scope="module")
def classifier():
    return WasteClassifier(MODEL_PATH)


def image_bytes(image_format, color="white"):
    buffer = io.BytesIO()
    Image.new("RGB", (224, 224), color).save(buffer, format=image_format)
    return buffer.getvalue()


def test_valid_image_is_classified(classifier):
    result = classifier.predict("ml/data/example.jpg")

    assert result["status"] == "ok"
    assert result["category"] in CATEGORIES
    assert 0 <= result["confidence"] <= 1


def test_corrupted_file_is_rejected(classifier):
    result = classifier.predict_bytes(b"this is not an image")

    assert result["status"] == "error"
    assert result["reason"] == "unreadable_image"
    assert result["can_sort"] is False


def test_unsupported_format_is_rejected(classifier):
    result = classifier.predict_bytes(image_bytes("GIF"))

    assert result["status"] == "error"
    assert result["reason"] == "unsupported_format"
    assert result["can_sort"] is False


def test_too_large_file_is_rejected(classifier):
    result = classifier.predict_bytes(b"0" * (MAX_FILE_SIZE_BYTES + 1))

    assert result["status"] == "error"
    assert result["reason"] == "file_too_large"
    assert result["can_sort"] is False


def test_low_confidence_cannot_be_sorted(classifier, monkeypatch):
    monkeypatch.setattr(classifier, "confidence_threshold", 1.0)

    result = classifier.predict_bytes(image_bytes("JPEG"))

    assert result["status"] == "ok"
    assert result["reason"] == "low_confidence"
    assert result["can_sort"] is False
