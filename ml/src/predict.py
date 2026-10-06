from pathlib import Path
from PIL import Image


def predict(image_path):
    with Image.open(image_path) as image:
        image = image.convert("RGB")

        print("Image loaded!")
        print("Size:", image.size)
        image.show()

    return {
        "category": "glass",
        "confidence": 0.97,
    }


if __name__ == "__main__":
    ml_folder = Path(__file__).resolve().parent.parent
    image_path = ml_folder / "data" / "example.jpg"

    result = predict(image_path)
    print(result)