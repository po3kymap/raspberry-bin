from pathlib import Path

# Folder with images
FOLDER = Path("")

# Class name
CLASS_NAME = ""
# Starting number
START_NUMBER = 38

# Supported formats
extensions = {".jpg", ".jpeg", ".png", ".webp"}

files = sorted(
    f for f in FOLDER.iterdir()
    if f.is_file() and f.suffix.lower() in extensions
)

# Temporary names to avoid conflicts
temp_files = []

for i, file in enumerate(files):
    temp_name = FOLDER / f"__temp_{i}{file.suffix.lower()}"
    file.rename(temp_name)
    temp_files.append((temp_name, file.suffix.lower()))

# Rename to final names
for i, (file, extension) in enumerate(temp_files):
    number = START_NUMBER + i
    new_name = FOLDER / f"{CLASS_NAME}_{number}{extension}"
    file.rename(new_name)

print(f"Renamed {len(files)} files.")
print(
    f"Range: {CLASS_NAME}_{START_NUMBER} → "
    f"{CLASS_NAME}_{START_NUMBER + len(files) - 1}"
)