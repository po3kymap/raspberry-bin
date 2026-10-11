#!/usr/bin/env python3
"""Inspect whether simple image metadata is strongly associated with labels."""

import argparse
import math
from collections import Counter, defaultdict
from pathlib import Path

from PIL import Image, ImageStat

DEFAULT_DATASET = Path(
    ""
)
CATEGORIES = ["GENERAL_WASTE", "ORGANIC", "PAPER", "PMD", "UNSUPPORTED"]
IMAGE_EXTENSIONS = {".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tif", ".tiff"}


def bucket_aspect_ratio(width, height):
    ratio = width / height
    if 0.9 <= ratio <= 1.1:
        return "Square"
    if ratio > 1.1:
        return "Landscape"
    return "Portrait"


def bucket_brightness(value):
    if value < 70:
        return "Dark"
    if value < 150:
        return "Medium"
    return "Bright"


def cramers_v(records, feature):
    """Measure association between a categorical feature and the label."""
    labels = sorted({row["label"] for row in records})
    values = sorted({row[feature] for row in records})
    if len(labels) < 2 or len(values) < 2:
        return 0.0

    table = [[0 for _ in values] for _ in labels]
    label_index = {value: index for index, value in enumerate(labels)}
    value_index = {value: index for index, value in enumerate(values)}
    for row in records:
        table[label_index[row["label"]]][value_index[row[feature]]] += 1

    row_totals = [sum(row) for row in table]
    column_totals = [sum(table[i][j] for i in range(len(labels))) for j in range(len(values))]
    total = len(records)
    if total == 0:
        return 0.0

    chi_squared = 0.0
    for i, row_total in enumerate(row_totals):
        for j, column_total in enumerate(column_totals):
            expected = row_total * column_total / total
            if expected:
                chi_squared += (table[i][j] - expected) ** 2 / expected

    denominator = total * min(len(labels) - 1, len(values) - 1)
    return math.sqrt(chi_squared / denominator) if denominator else 0.0


def scan_dataset(dataset):
    records = []
    unreadable = []
    for label in CATEGORIES:
        folder = dataset / label
        if not folder.is_dir():
            print(f"Warning: category folder not found: {folder}")
            continue

        for path in folder.rglob("*"):
            if not path.is_file() or path.suffix.lower() not in IMAGE_EXTENSIONS:
                continue
            try:
                with Image.open(path) as image:
                    image = image.convert("RGB")
                    width, height = image.size
                    mean_rgb = ImageStat.Stat(image).mean
                    brightness = sum(mean_rgb) / 3
                    saturation_proxy = max(mean_rgb) - min(mean_rgb)

                relative_parent = path.parent.relative_to(folder).as_posix()
                if relative_parent == ".":
                    relative_parent = "(category root)"

                records.append({
                    "label": label,
                    "extension": path.suffix.lower(),
                    "dimensions": f"{width}x{height}",
                    "aspect": bucket_aspect_ratio(width, height),
                    "brightness": bucket_brightness(brightness),
                    "colorfulness": "Low" if saturation_proxy < 35 else "Medium" if saturation_proxy < 85 else "High",
                    "source_folder": relative_parent,
                })
            except Exception as error:
                unreadable.append((path, str(error)))
    return records, unreadable


def print_distribution(records, feature, title, limit=8):
    print(f"\n{title}")
    by_label = defaultdict(Counter)
    totals = Counter()
    for row in records:
        by_label[row["label"]][row[feature]] += 1
        totals[row["label"]] += 1

    values = sorted({row[feature] for row in records})
    if feature in {"dimensions", "source_folder"}:
        values = [value for value, _ in Counter(row[feature] for row in records).most_common(limit)]
        if feature == "source_folder":
            print(f"  Showing up to {limit} most common subfolders overall.")
    print(f"  {'Category':<18}" + "".join(f"{value[:16]:>17}" for value in values))
    for label in CATEGORIES:
        total = totals[label]
        if not total:
            continue
        cells = "".join(f"{100 * by_label[label][value] / total:16.1f}%" for value in values)
        print(f"  {label:<18}{cells}")


def main():
    parser = argparse.ArgumentParser(
        description="Audit class imbalance and simple dataset source/style proxies."
    )
    parser.add_argument(
        "dataset",
        nargs="?",
        type=Path,
        default=DEFAULT_DATASET,
        help="Folder containing GENERAL_WASTE, ORGANIC, PAPER, PMD, and UNSUPPORTED.",
    )
    args = parser.parse_args()

    if not args.dataset.is_dir():
        raise SystemExit(f"Dataset folder not found: {args.dataset}")

    records, unreadable = scan_dataset(args.dataset)
    if not records:
        raise SystemExit("No readable images found in the category folders.")

    print(f"Dataset: {args.dataset}")
    print(f"Readable images: {len(records)}")
    print(f"Unreadable images skipped: {len(unreadable)}")

    counts = Counter(row["label"] for row in records)
    print("\nClass counts:")
    for label in CATEGORIES:
        print(f"  {label:<18}{counts[label]:>7} images")
    if counts:
        largest = max(counts.values())
        smallest = min((count for count in counts.values() if count), default=0)
        if smallest:
            print(f"  Largest-to-smallest class ratio: {largest / smallest:.2f}:1")

    features = [
        ("dimensions", "Image dimensions"),
        ("aspect", "Aspect ratio"),
        ("extension", "File format"),
        ("brightness", "Average brightness bucket"),
        ("colorfulness", "Average colorfulness proxy"),
        ("source_folder", "Nested source-folder names"),
    ]
    print("\nMetadata-to-label association (Cramer's V; 0 = little association, 1 = strong association):")
    for feature, title in features:
        print(f"  {title:<32} {cramers_v(records, feature):.3f}")

    print_distribution(records, "aspect", "Aspect ratio distribution (% within each class)")
    print_distribution(records, "brightness", "Brightness distribution (% within each class)")
    print_distribution(records, "extension", "File format distribution (% within each class)")
    print_distribution(records, "dimensions", "Most common dimensions (% within each class)")
    print_distribution(records, "source_folder", "Most common nested folders (% within each class)")

    if unreadable:
        print("\nFirst unreadable files:")
        for path, error in unreadable[:10]:
            print(f"  {path}: {error}")

    print(
        "\nInterpretation: strong association can reveal a possible shortcut or source imbalance, "
        "but it does not prove the image model relies on it. Color and dimensions can also reflect "
        "real differences between objects. Confirm concerns by reviewing samples and evaluating "
        "on new photos from the intended camera and setting."
    )


if __name__ == "__main__":
    main()
