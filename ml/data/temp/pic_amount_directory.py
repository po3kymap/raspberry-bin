from pathlib import Path

# Dataset folder
FOLDER = Path("/Users/artemnovohatnii/Desktop/uni/second_year/m5/project_dataset")

# Image formats
extensions = {".jpg", ".jpeg", ".png", ".webp"}

total = 0

print("Images by folder:")
print("-" * 30)

for folder in sorted(FOLDER.iterdir()):
    if not folder.is_dir():
        continue

    count = sum(
        1
        for file in folder.iterdir()
        if file.is_file() and file.suffix.lower() in extensions
    )

    total += count

    print(f"{folder.name:<25} {count}")

print("-" * 30)
print(f"Total images: {total}")