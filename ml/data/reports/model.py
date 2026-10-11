#!/usr/bin/env python3
"""Train and evaluate a MobileNetV3-Small waste-image classifier."""

import argparse
import random
from collections import Counter
from pathlib import Path

import torch
from PIL import Image
from torch import nn
from torch.utils.data import DataLoader, Dataset
from torchvision import models, transforms

DEFAULT_DATASET = Path(
    "/Users/artemnovohatnii/Desktop/uni/second_year/m5/project_dataset"
)
DEFAULT_MODEL = Path(
    "/Users/artemnovohatnii/Desktop/uni/second_year/m5/project/ml/models/mobilenetv3_waste_new.pth"
)
CATEGORIES = ["GENERAL_WASTE", "ORGANIC", "PAPER", "PMD", "UNSUPPORTED"]
IMAGE_EXTENSIONS = {".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tif", ".tiff"}
MEAN = (0.485, 0.456, 0.406)
STD = (0.229, 0.224, 0.225)


def collect_images(dataset_path):
    """Load readable images and their labels directly from category folders."""
    samples = []
    unreadable = 0

    for label_index, label in enumerate(CATEGORIES):
        folder = dataset_path / label
        if not folder.is_dir():
            raise FileNotFoundError(f"Category folder not found: {folder}")

        for path in folder.rglob("*"):
            if not path.is_file() or path.suffix.lower() not in IMAGE_EXTENSIONS:
                continue
            try:
                with Image.open(path) as image:
                    image.verify()
                samples.append({"path": path, "label": label_index})
            except Exception as error:
                unreadable += 1
                print(f"Skipping unreadable image: {path} ({error})")

    if not samples:
        raise SystemExit(f"No readable images found in: {dataset_path}")
    return samples, unreadable


def split_samples(samples, seed):
    """Create reproducible stratified train, validation, and test splits."""
    rng = random.Random(seed)
    split_names = ("train", "val", "test")
    assignments = {name: [] for name in split_names}

    for label_index in range(len(CATEGORIES)):
        class_samples = [sample for sample in samples if sample["label"] == label_index]
        if len(class_samples) < 3:
            raise SystemExit(
                f"Class {CATEGORIES[label_index]} needs at least 3 readable images to split."
            )
        rng.shuffle(class_samples)
        train_end = int(len(class_samples) * 0.70)
        val_end = train_end + int(len(class_samples) * 0.15)
        assignments["train"].extend(class_samples[:train_end])
        assignments["val"].extend(class_samples[train_end:val_end])
        assignments["test"].extend(class_samples[val_end:])

    for split in split_names:
        rng.shuffle(assignments[split])
    return assignments


class WasteImageDataset(Dataset):
    def __init__(self, samples, transform):
        self.items = [(sample["path"], sample["label"]) for sample in samples]
        self.transform = transform

    def __len__(self):
        return len(self.items)

    def __getitem__(self, index):
        path, label = self.items[index]
        with Image.open(path) as image:
            image = image.convert("RGB")
            return self.transform(image), label


def make_splits(samples, seed):
    grouped = split_samples(samples, seed)
    train_transform = transforms.Compose([
        transforms.RandomResizedCrop(224, scale=(0.75, 1.0)),
        transforms.RandomHorizontalFlip(),
        transforms.ColorJitter(brightness=0.15, contrast=0.15, saturation=0.15),
        transforms.ToTensor(),
        transforms.Normalize(MEAN, STD),
    ])
    evaluation_transform = transforms.Compose([
        transforms.Resize(256),
        transforms.CenterCrop(224),
        transforms.ToTensor(),
        transforms.Normalize(MEAN, STD),
    ])
    datasets = {
        "train": WasteImageDataset(grouped["train"], train_transform),
        "val": WasteImageDataset(grouped["val"], evaluation_transform),
        "test": WasteImageDataset(grouped["test"], evaluation_transform),
    }
    return grouped, datasets


def build_model(device):
    weights = models.MobileNet_V3_Small_Weights.DEFAULT
    model = models.mobilenet_v3_small(weights=weights)
    input_features = model.classifier[-1].in_features
    model.classifier[-1] = nn.Linear(input_features, len(CATEGORIES))
    return model.to(device)


def class_weights(dataset, device):
    counts = Counter(label for _, label in dataset.items)
    total = sum(counts.values())
    return torch.tensor(
        [total / max(len(CATEGORIES) * counts[index], 1) for index in range(len(CATEGORIES))],
        dtype=torch.float32,
        device=device,
    )


def run_epoch(model, loader, loss_function, device, optimizer=None):
    training = optimizer is not None
    model.train(training)
    total_loss = 0.0
    correct = 0
    total = 0
    all_predictions = []
    all_labels = []

    for images, labels in loader:
        images = images.to(device)
        labels = labels.to(device)
        if training:
            optimizer.zero_grad(set_to_none=True)

        with torch.set_grad_enabled(training):
            logits = model(images)
            loss = loss_function(logits, labels)
            if training:
                loss.backward()
                optimizer.step()

        predictions = logits.argmax(dim=1)
        batch_size = labels.size(0)
        total_loss += loss.item() * batch_size
        correct += (predictions == labels).sum().item()
        total += batch_size
        all_predictions.extend(predictions.detach().cpu().tolist())
        all_labels.extend(labels.detach().cpu().tolist())

    return total_loss / max(total, 1), correct / max(total, 1), all_predictions, all_labels


def print_metrics(predictions, labels):
    matrix = [[0 for _ in CATEGORIES] for _ in CATEGORIES]
    for truth, prediction in zip(labels, predictions):
        matrix[truth][prediction] += 1

    total = len(labels)
    accuracy = sum(matrix[index][index] for index in range(len(CATEGORIES))) / max(total, 1)
    recalls = []
    f1_values = []
    for index in range(len(CATEGORIES)):
        true_positive = matrix[index][index]
        actual = sum(matrix[index])
        predicted = sum(matrix[row][index] for row in range(len(CATEGORIES)))
        precision = true_positive / predicted if predicted else 0.0
        recall = true_positive / actual if actual else 0.0
        recalls.append(recall)
        f1_values.append(2 * precision * recall / (precision + recall) if precision + recall else 0.0)

    print(f"Test accuracy:          {accuracy:.4f}")
    print(f"Test balanced accuracy: {sum(recalls) / len(recalls):.4f}")
    print(f"Test macro F1:          {sum(f1_values) / len(f1_values):.4f}")
    print("\nConfusion matrix (rows = true class, columns = predicted class):")
    print(f"{'':<18}" + "".join(f"{name[:12]:>14}" for name in CATEGORIES))
    for index, row in enumerate(matrix):
        print(f"{CATEGORIES[index]:<18}" + "".join(f"{value:>14}" for value in row))

    print("\nPer-class results:")
    for index, name in enumerate(CATEGORIES):
        actual = sum(matrix[index])
        predicted = sum(matrix[row][index] for row in range(len(CATEGORIES)))
        precision = matrix[index][index] / predicted if predicted else 0.0
        print(
            f"  {name:<18} precision={precision:.3f} recall={recalls[index]:.3f} "
            f"F1={f1_values[index]:.3f} test_images={actual}"
        )


def main():
    parser = argparse.ArgumentParser(description="Train and evaluate a five-class waste classifier.")
    parser.add_argument("--dataset", type=Path, default=DEFAULT_DATASET)
    parser.add_argument("--model-out", type=Path, default=DEFAULT_MODEL)
    parser.add_argument("--epochs", type=int, default=8)
    parser.add_argument("--batch-size", type=int, default=32)
    parser.add_argument("--seed", type=int, default=42)
    args = parser.parse_args()

    if args.epochs < 1 or args.batch_size < 1:
        raise SystemExit("Epochs and batch size must be positive integers.")

    random.seed(args.seed)
    torch.manual_seed(args.seed)
    if torch.cuda.is_available():
        device = torch.device("cuda")
    elif torch.backends.mps.is_available():
        device = torch.device("mps")
    else:
        device = torch.device("cpu")
    print(f"Using device: {device}")

    samples, unreadable = collect_images(args.dataset)
    grouped, datasets = make_splits(samples, args.seed)
    print(f"Readable images: {len(samples)}")
    print(f"Unreadable images skipped: {unreadable}")
    for split_name in grouped:
        print(f"{split_name.capitalize()} images: {len(datasets[split_name])}")

    workers = 0
    loaders = {
        "train": DataLoader(datasets["train"], batch_size=args.batch_size, shuffle=True, num_workers=workers),
        "val": DataLoader(datasets["val"], batch_size=args.batch_size, shuffle=False, num_workers=workers),
        "test": DataLoader(datasets["test"], batch_size=args.batch_size, shuffle=False, num_workers=workers),
    }

    model = build_model(device)
    loss_function = nn.CrossEntropyLoss(weight=class_weights(datasets["train"], device))
    optimizer = torch.optim.AdamW(model.parameters(), lr=0.0001, weight_decay=0.0001)
    best_validation_loss = float("inf")
    best_state = None

    for epoch in range(1, args.epochs + 1):
        train_loss, train_accuracy, _, _ = run_epoch(
            model, loaders["train"], loss_function, device, optimizer
        )
        validation_loss, validation_accuracy, _, _ = run_epoch(
            model, loaders["val"], loss_function, device
        )
        print(
            f"Epoch {epoch}/{args.epochs}: "
            f"train_loss={train_loss:.4f}, train_accuracy={train_accuracy:.4f}, "
            f"val_loss={validation_loss:.4f}, val_accuracy={validation_accuracy:.4f}"
        )
        if validation_loss < best_validation_loss:
            best_validation_loss = validation_loss
            best_state = {key: value.detach().cpu().clone() for key, value in model.state_dict().items()}

    if best_state is None:
        raise RuntimeError("Training did not produce a valid model checkpoint.")
    model.load_state_dict(best_state)
    test_loss, _, predictions, labels = run_epoch(model, loaders["test"], loss_function, device)
    print(f"\nBest validation loss: {best_validation_loss:.4f}")
    print(f"Test loss:            {test_loss:.4f}")
    print_metrics(predictions, labels)

    args.model_out.parent.mkdir(parents=True, exist_ok=True)
    torch.save(
        {"state_dict": model.state_dict(), "categories": CATEGORIES, "image_size": 224},
        args.model_out,
    )
    print(f"\nSaved model to: {args.model_out}")
    print("The test split was not used for training or model selection.")


if __name__ == "__main__":
    main()
