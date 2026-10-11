from ml.data.scripts.predict import load_model, choose_device

class WasteClassifier:
    def __init__(self, model_path):
        self.model_path = model_path
        self.device = choose_device()
        self.model, self.categories = load_model(self.model_path, self.device)
        
        
if __name__ == "__main__":
    classifier = WasteClassifier("ml/models/mobilenetv3_waste.pth")
    