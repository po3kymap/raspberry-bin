import torch
from PIL import Image, UnidentifiedImageError
from ml.data.scripts.predict import load_model, choose_device, get_transform


class WasteClassifier:
    def __init__(self, model_path):
        self.model_path = model_path
        self.device = choose_device()
        self.model, self.categories = load_model(self.model_path, self.device)
        self.preprocess = get_transform()
     
     
    def predict(self, image_path):
        try:
            
            with Image.open(image_path) as image:
                image = image.convert("RGB")
                tensor = self.preprocess(image).unsqueeze(0)
        except FileNotFoundError:
            return {
                "status": "error",
                "message": "Image file not found.",
                "can_sort": False,
            } 
            
        except (UnidentifiedImageError, OSError):
            return {
                "status": "error",
                "message": "Image could not be read.",
                "can_sort": False,
            }
            
        tensor = tensor.to(self.device)

        with torch.inference_mode():
            scores = self.model(tensor)
            probabilities = torch.softmax(scores, dim=1)[0]

        index = probabilities.argmax().item()

        return {
            "status": "ok",
            "category": self.categories[index],
            "confidence": probabilities[index].item(),
        }
        
           
        
if __name__ == "__main__":
    classifier = WasteClassifier("ml/models/mobilenetv3_waste_sd.pth")
    print("Device:", classifier.device)
    print("Categories: ", classifier.categories)
    result = classifier.predict("ml/data/example.jpg")
    print(result)
    