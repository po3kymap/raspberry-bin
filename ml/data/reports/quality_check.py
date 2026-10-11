from pathlib import Path
from PIL import Image, ImageStat

# Dataset folder
FOLDER = Path("/")

# Supported image formats
EXTENSIONS = {".jpg", ".jpeg", ".png", ".webp"}

# Minimum acceptable resolution
MIN_WIDTH = 100
MIN_HEIGHT = 100

# Thresholds for almost-black / almost-white images
BLACK_THRESHOLD = 5
WHITE_THRESHOLD = 250


def is_almost_black(image):
    grayscale = image.convert("L")
    mean = ImageStat.Stat(grayscale).mean[0]
    return mean < BLACK_THRESHOLD


def is_almost_white(image):
    grayscale = image.convert("L")
    mean = ImageStat.Stat(grayscale).mean[0]
    return mean > WHITE_THRESHOLD


total = 0
valid = 0

corrupted = []
too_small = []
almost_black = []
almost_white = []
unsupported_format = []

print("Starting dataset quality check...")
print()

for folder in sorted(FOLDER.iterdir()):

    if not folder.is_dir():
        continue

    print(f"Checking: {folder.name}")

    files = [
        file for file in folder.iterdir()
        if file.is_file()
    ]

    for index, file in enumerate(files, start=1):

        total += 1

        # Check file extension

        if file.suffix.lower() not in EXTENSIONS:
            unsupported_format.append(file)
            continue

        try:

            # Check whether image is valid

            with Image.open(file) as image:
                image.verify()

            # Re-open after verify()

            with Image.open(file) as image:

                width, height = image.size

                # Check resolution
                if width < MIN_WIDTH or height < MIN_HEIGHT:
                    too_small.append(
                        (file, width, height)
                    )

                # Check almost-black
                if is_almost_black(image):
                    almost_black.append(file)

                # Check almost-white
                if is_almost_white(image):
                    almost_white.append(file)

                valid += 1

        except Exception as error:

            corrupted.append(
                (file, str(error))
            )
            # Progress

        if index % 500 == 0:
            print(f"  Processed {index}/{len(files)}")

print()
print("=" * 50)
print("DATASET QUALITY REPORT")
print("=" * 50)

print(f"Total files:             {total}")
print(f"Valid images:            {valid}")
print(f"Corrupted images:        {len(corrupted)}")
print(f"Too small:               {len(too_small)}")
print(f"Almost black:            {len(almost_black)}")
print(f"Almost white:            {len(almost_white)}")
print(f"Unsupported format:      {len(unsupported_format)}")

print("=" * 50)

if corrupted:

    print("\nCORRUPTED:")

    for file, error in corrupted:
        print(f"  {file}")
        print(f"    Error: {error}")

if too_small:

    print("\nTOO SMALL:")

    for file, width, height in too_small:
        print(f"  {file} ({width}x{height})")

if almost_black:

    print("\nALMOST BLACK:")

    for file in almost_black:
        print(f"  {file}")

if almost_white:

    print("\nALMOST WHITE:")

    for file in almost_white:
        print(f"  {file}")

if unsupported_format:

    print("\nUNSUPPORTED FORMAT:")

    for file in unsupported_format:
        print(f"  {file}")

files_to_delete = []

# Too small
for file, width, height in too_small:
    files_to_delete.append(file)

# Almost black
for file in almost_black:
    files_to_delete.append(file)

# Unsupported formats
for file in unsupported_format:
    files_to_delete.append(file)

# Remove duplicates from deletion list
files_to_delete = list(set(files_to_delete))

print()
print("=" * 50)
print("CLEANUP")
print("=" * 50)

print(f"Files to delete: {len(files_to_delete)}")

# Delete files
for file in files_to_delete:

    try:
        file.unlink()
        print(f"Deleted: {file}")

    except Exception as error:
        print(f"Could not delete {file}")
        print(f"  Error: {error}")

print(f"Deleted files: {len(files_to_delete)}")
print(f"Almost-white files kept: {len(almost_white)}")