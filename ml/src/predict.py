from pathlib import Path
from PIL import Image, UnidentifiedImageError


def load_image(image_path):
    with Image.open(image_path) as image:
        return image.convert("RGB")
    
    
    
def predict(image_path):
    try:
        image = load_image(image_path)
    except FileNotFoundError:
        return {
            "status": "error",
            "comment": "Image was not found",
            "can_sort": False
        }
    except (UnidentifiedImageError, OSError):
        return {
            "status": "error",
            "message": "Image could not be read.",
            "can_sort": False
        }
    print("Image loaded!")
    print("Size:", image.size)
    image.show()

    return {
        "category": "paper",
        "confidence": 0.87,
        "can_sort": True,
    }


if __name__ == "__main__":
    ml_folder = Path(__file__).resolve().parent.parent
    image_path = ml_folder / "data" / "exampl1e.jpg"

    result = predict(image_path)
    print(result)