#!/usr/bin/env python3
"""Run a saved MobileNetV3-Small waste classifier on image files."""

import argparse
from pathlib import Path

import torch
from PIL import Image
from torchvision import models, transforms

DEFAULT_MODEL = Path(
    "/Users/artemnovohatnii/Desktop/uni/second_year/m5/project/ml/models/mobilenetv3_waste_new.pth"
)
# Change this path to the image file or folder you want to classify.
IMAGE_PATH = Path("/Users/artemnovohatnii/Downloads/save/_10.jpeg")
IMAGE_EXTENSIONS = {".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tif", ".tiff"}
MEAN = (0.485, 0.456, 0.406)
STD = (0.229, 0.224, 0.225)


def choose_device():
    if torch.cuda.is_available():
        return torch.device("cuda")
    if torch.backends.mps.is_available():
        return torch.device("mps")
    return torch.device("cpu")


def load_model(model_path, device):
    checkpoint = torch.load(model_path, map_location="cpu", weights_only=True)
    categories = checkpoint["categories"]
    model = models.mobilenet_v3_small(weights=None)
    input_features = model.classifier[-1].in_features
    model.classifier[-1] = torch.nn.Linear(input_features, len(categories))
    model.load_state_dict(checkpoint["state_dict"])
    model.eval()
    return model.to(device), categories


def get_transform():
    return transforms.Compose([
        transforms.Resize(256),
        transforms.CenterCrop(224),
        transforms.ToTensor(),
        transforms.Normalize(MEAN, STD),
    ])


def find_images(input_path):
    if input_path.is_file():
        if input_path.suffix.lower() not in IMAGE_EXTENSIONS:
            raise SystemExit(f"Unsupported image format: {input_path}")
        return [input_path]
    if input_path.is_dir():
        return sorted(
            path for path in input_path.rglob("*")
            if path.is_file() and path.suffix.lower() in IMAGE_EXTENSIONS
        )
    raise SystemExit(f"Input path does not exist: {input_path}")


def main():
    parser = argparse.ArgumentParser(
        description="Predict waste category and show model confidence for image files."
    )
    parser.add_argument(
        "input",
        type=Path,
        nargs="?",
        default=IMAGE_PATH,
        help="Optional image file or folder. If omitted, IMAGE_PATH in this script is used.",
    )
    parser.add_argument("--model", type=Path, default=DEFAULT_MODEL, help="Trained model checkpoint.")
    parser.add_argument("--top", type=int, default=5, help="Number of class probabilities to show.")
    args = parser.parse_args()

    if not args.model.is_file():
        raise SystemExit(f"Model file not found: {args.model}")
    if args.top < 1:
        raise SystemExit("--top must be at least 1.")

    device = choose_device()
    print(f"Using device: {device}")
    model, categories = load_model(args.model, device)
    transform = get_transform()
    image_paths = find_images(args.input)
    if not image_paths:
        raise SystemExit(f"No supported images found in: {args.input}")

    for image_path in image_paths:
        try:
            with Image.open(image_path) as image:
                tensor = transform(image.convert("RGB")).unsqueeze(0).to(device)
            with torch.inference_mode():
                probabilities = torch.softmax(model(tensor), dim=1)[0].cpu()
            ranking = torch.argsort(probabilities, descending=True)[:min(args.top, len(categories))]
            prediction = categories[int(ranking[0])]
            confidence = float(probabilities[int(ranking[0])]) * 100

            print(f"\nImage: {image_path}")
            print(f"Prediction: {prediction}")
            print(f"Model confidence estimate: {confidence:.2f}%")
            print("Class probabilities:")
            for index in ranking:
                class_name = categories[int(index)]
                percent = float(probabilities[int(index)]) * 100
                print(f"  {class_name:<18} {percent:6.2f}%")
        except Exception as error:
            print(f"\nCould not process {image_path}: {error}")


if __name__ == "__main__":
    main()
