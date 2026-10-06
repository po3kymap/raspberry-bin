
import torch
from pathlib import Path
from PIL import Image, UnidentifiedImageError
from torchvision.models import MobileNet_V3_Small_Weights

preprocess = MobileNet_V3_Small_Weights.DEFAULT.transforms()
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
            "comment": "Image could not be read.",
            "can_sort": False
        }
    print("Image loaded!")
    print("Size:", image.size)
    image_proc: torch.Tensor = preprocess(image)
    image_proc = image_proc.unsqueeze(0)

    print("Tensor shape:", image_proc.shape)
    image.show()

    return {
        "category": "paper",
        "confidence": 0.87,
        "can_sort": False,
    }


if __name__ == "__main__":
    ml_folder = Path(__file__).resolve().parent.parent
    image_path = ml_folder / "data" / "example.jpg"

    result = predict(image_path)
    print(result)