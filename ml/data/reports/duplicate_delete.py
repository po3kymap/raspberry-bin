from pathlib import Path
from collections import defaultdict
from hashlib import sha256

DATASET_FOLDER = Path(
    "/"
)


def get_file_hash(file_path):
    file_hash = sha256()

    with file_path.open("rb") as file:
        for chunk in iter(lambda: file.read(1024 * 1024), b""):
            file_hash.update(chunk)

    return file_hash.hexdigest()


def main():
    files_by_hash = defaultdict(list)

    for file_path in DATASET_FOLDER.rglob("*"):
        if not file_path.is_file():
            continue

        try:
            file_hash = get_file_hash(file_path)
            files_by_hash[file_hash].append(file_path)
        except OSError as error:
            print(f"Could not read {file_path}: {error}")

    deleted_count = 0
    skipped_groups = 0

    for duplicate_group in files_by_hash.values():
        if len(duplicate_group) < 2:
            continue

        duplicate_group.sort()
        labels = {file_path.parent.name for file_path in duplicate_group}

        print("\nDuplicates found:")
        for file_path in duplicate_group:
            print(f"  {file_path}")

        if len(labels) > 1:
            print("Skipped: files have different folder labels.")
            skipped_groups += 1
            continue

        # Keep the first file and delete the remaining identical copies
        for file_path in duplicate_group[1:]:
            try:
                file_path.unlink()
                deleted_count += 1
                print(f"Deleted: {file_path}")
            except OSError as error:
                print(f"Could not delete {file_path}: {error}")

    print(f"\nDuplicate files deleted: {deleted_count}")
    print(f"Groups skipped because of different labels: {skipped_groups}")


if __name__ == "__main__":
    main()