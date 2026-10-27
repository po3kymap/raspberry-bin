#!/usr/bin/env python3
"""Train the Raspberry Bin waste classifier.

Examples
--------
# internet data only
python ml/src/train.py --dataset ~/Desktop/uni/second_year/m5/project_dataset_clean

# internet data + own photos from the box (recommended)
python ml/src/train.py --dataset .../project_dataset_clean --own .../project_dataset_own --arch mobilenet_v3_large

# DINOv2 ViT-S/14 (needs: pip install timm)
python ml/src/train.py --dataset ../project_dataset_clean --arch dinov2_vits14

Every run gets its own folder: ml/models/train, ml/models/train2, ml/models/train3, ...

Own photos: same class folders (GENERAL_WASTE, ORGANIC, PAPER, PMD, UNSUPPORTED).
Name files <anything>_<item>_<nr>.jpg, e.g. PMD_cola_can_03.jpg: everything before the last
"_<number>" is the item id. Train/val/test for own photos are split BY ITEM, so the test
measures items the model has never seen.
"""

import argparse
import io
import json
import math
import random
import re
import time
from collections import Counter, defaultdict
from pathlib import Path

import numpy as np
import torch
from PIL import Image, ImageFilter, ImageOps
from torch import nn
from torch.utils.data import DataLoader, Dataset, WeightedRandomSampler
from torchvision import models, transforms


# ---------------------------------------------------------------- preprocessing & models

CATEGORIES = ["GENERAL_WASTE", "ORGANIC", "PAPER", "PMD", "UNSUPPORTED"]
REJECT_CLASS = "UNSUPPORTED"
IMAGE_EXTENSIONS = {".jpg", ".jpeg", ".png", ".webp", ".bmp"}
MEAN = (0.485, 0.456, 0.406)
STD = (0.229, 0.224, 0.225)
ARCHS = ("mobilenet_v3_small", "mobilenet_v3_large", "efficientnet_b0", "convnext_small",
         "dinov2_vits14", "dinov2_vitb14")
# DINOv2 backbones come from timm (pip install timm); weights are downloaded from the Hugging Face hub.
DINOV2_TIMM_NAMES = {
    "dinov2_vits14": "vit_small_patch14_dinov2.lvd142m",
    "dinov2_vitb14": "vit_base_patch14_dinov2.lvd142m",
}
# Modern CNNs from timm: pretrained on ImageNet-22k, then fine-tuned on ImageNet-1k (best for transfer).
TIMM_CNN_NAMES = {
    "convnext_small": "convnext_small.fb_in22k_ft_in1k",
}


def list_images(folder):
    folder = Path(folder)
    return sorted(
        p for p in folder.rglob("*")
        if p.is_file() and p.suffix.lower() in IMAGE_EXTENSIONS and not p.name.startswith(".")
    )


def load_rgb(path):
    with Image.open(path) as image:
        return ImageOps.exif_transpose(image).convert("RGB")


class PadToSquare:
    """Pad to a square using the image's own border colour (no grey bars that could act as a class hint)."""

    def __call__(self, image):
        w, h = image.size
        if w == h:
            return image
        a = np.asarray(image)
        border = np.concatenate([a[0], a[-1], a[:, 0], a[:, -1]])
        colour = tuple(int(v) for v in np.median(border, axis=0))
        side = max(w, h)
        canvas = Image.new("RGB", (side, side), colour)
        canvas.paste(image, ((side - w) // 2, (side - h) // 2))
        return canvas


class RandomRot90:
    def __call__(self, image):
        return image.rotate(90 * random.randint(0, 3), expand=True)


class RandomJpeg:
    """Simulate camera / compression artefacts."""

    def __init__(self, p=0.3, quality=(35, 90)):
        self.p, self.quality = p, quality

    def __call__(self, image):
        if random.random() > self.p:
            return image
        buffer = io.BytesIO()
        image.save(buffer, "JPEG", quality=random.randint(*self.quality))
        buffer.seek(0)
        return Image.open(buffer).convert("RGB")


class RandomBlur:
    def __init__(self, p=0.2, radius=(0.3, 1.5)):
        self.p, self.radius = p, radius

    def __call__(self, image):
        if random.random() > self.p:
            return image
        return image.filter(ImageFilter.GaussianBlur(random.uniform(*self.radius)))


def train_transform(size=224):
    return transforms.Compose([
        RandomRot90(),
        PadToSquare(),
        transforms.RandomResizedCrop(size, scale=(0.55, 1.0), ratio=(0.8, 1.25)),
        transforms.RandomHorizontalFlip(),
        transforms.ColorJitter(brightness=0.3, contrast=0.3, saturation=0.3, hue=0.03),
        RandomBlur(),
        RandomJpeg(),
        transforms.ToTensor(),
        transforms.Normalize(MEAN, STD),
    ])


def eval_transform(size=224):
    return transforms.Compose([
        PadToSquare(),
        transforms.Resize((size, size)),
        transforms.ToTensor(),
        transforms.Normalize(MEAN, STD),
    ])


def build_model(arch, num_classes, pretrained=True, image_size=224):
    if arch in DINOV2_TIMM_NAMES:
        import timm
        # DINOv2 is pretrained at 518 px; timm interpolates the position embeddings to image_size
        # (must be a multiple of the 14 px patch, e.g. 224).
        return timm.create_model(DINOV2_TIMM_NAMES[arch], pretrained=pretrained, num_classes=num_classes,
                                 img_size=image_size, drop_path_rate=0.1)
    if arch in TIMM_CNN_NAMES:
        import timm
        return timm.create_model(TIMM_CNN_NAMES[arch], pretrained=pretrained, num_classes=num_classes,
                                 drop_path_rate=0.2)
    if arch == "mobilenet_v3_small":
        model = models.mobilenet_v3_small(weights="DEFAULT" if pretrained else None)
        model.classifier[-1] = nn.Linear(model.classifier[-1].in_features, num_classes)
    elif arch == "mobilenet_v3_large":
        model = models.mobilenet_v3_large(weights="DEFAULT" if pretrained else None)
        model.classifier[-1] = nn.Linear(model.classifier[-1].in_features, num_classes)
    elif arch == "efficientnet_b0":
        model = models.efficientnet_b0(weights="DEFAULT" if pretrained else None)
        model.classifier[-1] = nn.Linear(model.classifier[-1].in_features, num_classes)
    else:
        raise ValueError(f"Unknown arch {arch}; choose from {ARCHS}")
    return model


def param_groups(model, arch, lr, weight_decay, layer_decay=0.7):
    """AdamW parameter groups: no weight decay on norm/bias/tokens.

    DINOv2: layer-wise LR decay, the head gets `lr`, every transformer block below it gets
    `layer_decay` times less, so the pretrained early layers barely move.
    CNNs: the new final layer gets `lr`, the pretrained body 0.3 * lr
    (torchvision calls that layer classifier.*, timm head.fc.*).
    """
    if arch in DINOV2_TIMM_NAMES:
        depth = len(model.blocks)

        def layer(name):
            if name.startswith(("cls_token", "pos_embed", "reg_token", "patch_embed")):
                return 0
            if name.startswith("blocks."):
                return int(name.split(".")[1]) + 1
            return depth + 1  # final norm + head

        def scale(name):
            return layer_decay ** (depth + 1 - layer(name))
    else:
        def scale(name):
            return 1.0 if name.startswith(("classifier", "head.fc")) else 0.3

    groups = {}
    for name, p in model.named_parameters():
        if not p.requires_grad:
            continue
        no_decay = p.ndim <= 1 or name.endswith(".bias") or name in ("cls_token", "pos_embed", "reg_token")
        key = (scale(name), no_decay)
        group = groups.setdefault(key, {"params": [], "lr": lr * key[0],
                                        "weight_decay": 0.0 if no_decay else weight_decay})
        group["params"].append(p)
    return list(groups.values())


MODELS_DIR = Path(__file__).resolve().parents[1] / "models"
DEFAULT_DATASET = Path(__file__).resolve().parents[3] / "project_dataset_clean"


# ---------------------------------------------------------------- data

# Name used in the saved files: model_<NAME>.pth, report_<NAME>.json, errors_<NAME>.tsv
DISPLAY_NAMES = {
    "mobilenet_v3_small": "MobileNetV3-Small",
    "mobilenet_v3_large": "MobileNetV3-Large",
    "efficientnet_b0": "EfficientNet-B0",
    "convnext_small": "ConvNeXt-Small",
    "dinov2_vits14": "DINOv2-ViT-S-14",
    "dinov2_vitb14": "DINOv2-ViT-B-14",
}

def output_paths(folder, name):
    """model_<name>.pth etc.; if that name is taken, use <name>_2, <name>_3 ... (never overwrite)."""
    folder = Path(folder)
    candidate, i = name, 2
    while (folder / f"model_{candidate}.pth").exists() or (folder / f"report_{candidate}.json").exists():
        candidate, i = f"{name}_{i}", i + 1
    return {"model": folder / f"model_{candidate}.pth",
            "report": folder / f"report_{candidate}.json",
            "errors": folder / f"errors_{candidate}.tsv"}


def item_id(path):
    stem = Path(path).stem
    return re.sub(r"[_\- ]?\(?\d+\)?$", "", stem).lower() or stem.lower()


def collect(root, source):
    samples = []
    for label, name in enumerate(CATEGORIES):
        folder = Path(root) / name
        if not folder.is_dir():
            if source == "own":
                continue
            raise SystemExit(f"Missing class folder: {folder}")
        for path in list_images(folder):
            group = f"{name}/{item_id(path)}" if source == "own" else f"{name}/{path.name}"
            samples.append({"path": str(path), "label": label, "source": source, "group": group})
    return samples


def grouped_split(samples, fractions, seed):
    """Stratified by class, grouped by `group` so one item never sits in two splits."""
    rng = random.Random(seed)
    out = {"train": [], "val": [], "test": []}
    by_class = defaultdict(lambda: defaultdict(list))
    for s in samples:
        by_class[s["label"]][s["group"]].append(s)
    for groups in by_class.values():
        keys = sorted(groups)
        rng.shuffle(keys)
        n = len(keys)
        n_val = max(1, round(n * fractions[1])) if n >= 3 else 0
        n_test = max(1, round(n * fractions[2])) if n >= 3 else 0
        parts = {"test": keys[:n_test], "val": keys[n_test:n_test + n_val], "train": keys[n_test + n_val:]}
        for split, ks in parts.items():
            for k in ks:
                out[split].extend(groups[k])
    for v in out.values():
        rng.shuffle(v)
    return out


class Images(Dataset):
    def __init__(self, samples, transform):
        self.samples, self.transform = samples, transform

    def __len__(self):
        return len(self.samples)

    def __getitem__(self, i):
        s = self.samples[i]
        return self.transform(load_rgb(s["path"])), s["label"], i


def balanced_sampler(samples, own_weight):
    """Every class is drawn equally often; inside a class an own photo is `own_weight`x more likely
    than an internet photo."""
    raw = [own_weight if s["source"] == "own" else 1.0 for s in samples]
    per_class = Counter()
    for s, r in zip(samples, raw):
        per_class[s["label"]] += r
    weights = [r / per_class[s["label"]] for s, r in zip(samples, raw)]
    return WeightedRandomSampler(weights, num_samples=len(samples), replacement=True)


# ---------------------------------------------------------------- metrics

def confusion(labels, preds, k=len(CATEGORIES)):
    m = [[0] * k for _ in range(k)]
    for t, p in zip(labels, preds):
        m[t][p] += 1
    return m


def summarize(labels, preds):
    m = confusion(labels, preds)
    k = len(m)
    total = sum(map(sum, m)) or 1
    per = {}
    f1s, recs = [], []
    for i in range(k):
        tp = m[i][i]
        actual = sum(m[i])
        pred = sum(m[r][i] for r in range(k))
        p = tp / pred if pred else 0.0
        r = tp / actual if actual else 0.0
        f = 2 * p * r / (p + r) if p + r else 0.0
        if actual:
            f1s.append(f)
            recs.append(r)
        per[CATEGORIES[i]] = {"precision": round(p, 4), "recall": round(r, 4), "f1": round(f, 4), "n": actual}
    return {
        "n": total if labels else 0,
        "accuracy": round(sum(m[i][i] for i in range(k)) / total, 4),
        "balanced_accuracy": round(sum(recs) / len(recs), 4) if recs else 0.0,
        "macro_f1": round(sum(f1s) / len(f1s), 4) if f1s else 0.0,
        "per_class": per,
        "confusion": m,
    }


def threshold_table(labels, probs):
    """Accuracy among accepted items vs. how many get rejected, for a range of thresholds."""
    rej = CATEGORIES.index(REJECT_CLASS)
    rows = []
    for t in [round(x * 0.05, 2) for x in range(6, 20)]:
        accepted = correct = sup = sup_rejected = uns = uns_caught = 0
        for y, pr in zip(labels, probs):
            conf, p = max((v, i) for i, v in enumerate(pr))
            reject = p == rej or conf < t
            if y == rej:
                uns += 1
                uns_caught += reject
            else:
                sup += 1
                sup_rejected += reject
            if not reject:
                accepted += 1
                correct += p == y
        rows.append({
            "threshold": t,
            "accuracy_on_accepted": round(correct / accepted, 4) if accepted else 0.0,
            "supported_rejected": round(sup_rejected / sup, 4) if sup else 0.0,
            "unsupported_caught": round(uns_caught / uns, 4) if uns else 0.0,
        })
    return rows


def pick_threshold(rows, max_reject):
    ok = [r for r in rows if r["supported_rejected"] <= max_reject]
    if not ok:
        return rows[0]
    return max(ok, key=lambda r: (r["accuracy_on_accepted"], -r["threshold"]))


# ---------------------------------------------------------------- loop

def amp_dtype(device):
    if device.type != "cuda":
        return None
    return torch.bfloat16 if torch.cuda.is_bf16_supported() else torch.float16


@torch.no_grad()
def predict(model, loader, device):
    model.eval()
    labels, preds, probs, idxs = [], [], [], []
    dtype = amp_dtype(device)
    for x, y, i in loader:
        with torch.autocast(device.type, dtype=dtype, enabled=dtype is not None):
            logits = model(x.to(device, non_blocking=True))
        p = torch.softmax(logits.float(), 1).cpu()
        labels += y.tolist()
        preds += p.argmax(1).tolist()
        probs += p.tolist()
        idxs += i.tolist()
    return labels, preds, probs, idxs


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--dataset", type=Path, default=DEFAULT_DATASET,
                    help="cleaned internet dataset (class folders); default: ../project_dataset_clean")
    ap.add_argument("--own", type=Path, help="own photos from the box (class folders)")
    ap.add_argument("--own-weight", type=float, default=5.0, help="how much more often own photos are sampled")
    ap.add_argument("--out", type=Path, default=MODELS_DIR, help="folder for the saved files (default: ml/models)")
    ap.add_argument("--name", help="name in the file names (default: from --arch, e.g. DINOv2-ViT-S-14)")
    ap.add_argument("--arch", choices=ARCHS, default="dinov2_vits14")
    ap.add_argument("--epochs", type=int, default=25)
    ap.add_argument("--batch-size", type=int, default=48)
    ap.add_argument("--lr", type=float, default=None,
                    help="head learning rate (default 5e-4 for MobileNet/EfficientNet, 2e-4 for ConvNeXt, "
                         "3e-4 for DINOv2)")
    ap.add_argument("--layer-decay", type=float, default=0.75,
                    help="DINOv2 only: each transformer block below the head learns this much slower")
    ap.add_argument("--image-size", type=int, default=224)
    ap.add_argument("--workers", type=int, default=4)
    ap.add_argument("--max-reject", type=float, default=0.10,
                    help="max share of supported items the threshold may reject (on validation)")
    ap.add_argument("--limit", type=int, default=0, help="debug: use at most N images per class")
    ap.add_argument("--seed", type=int, default=42)
    args = ap.parse_args()
    is_vit = args.arch in DINOV2_TIMM_NAMES
    if args.lr is None:
        args.lr = 3e-4 if is_vit else 2e-4 if args.arch == "convnext_small" else 5e-4
    if is_vit and args.image_size % 14:
        raise SystemExit("DINOv2 needs --image-size divisible by 14 (e.g. 224 or 266).")

    random.seed(args.seed)
    torch.manual_seed(args.seed)
    device = torch.device("cuda" if torch.cuda.is_available()
                          else "mps" if torch.backends.mps.is_available() else "cpu")
    args.out.mkdir(parents=True, exist_ok=True)
    paths = output_paths(args.out, args.name or DISPLAY_NAMES[args.arch])
    print(f"will save: {paths['model']}")

    internet = collect(args.dataset, "internet")
    own = collect(args.own, "own") if args.own else []
    if args.limit:
        rng = random.Random(args.seed)
        internet = [s for c in range(len(CATEGORIES))
                    for s in rng.sample([x for x in internet if x["label"] == c],
                                        min(args.limit, sum(x["label"] == c for x in internet)))]
    split_i = grouped_split(internet, (0.80, 0.10, 0.10), args.seed)
    split_o = grouped_split(own, (0.60, 0.20, 0.20), args.seed) if own else {"train": [], "val": [], "test": []}
    train = split_i["train"] + split_o["train"]

    print(f"device={device} arch={args.arch}")
    for name, s in [("internet", split_i), ("own", split_o)]:
        print(f"{name:9} " + "  ".join(f"{k}={len(v)}" for k, v in s.items()))
    if own:
        items = Counter(g.split("/")[0] for g in {s["group"] for s in own})
        print("own photos per class:", dict(Counter(CATEGORIES[s["label"]] for s in own)))
        print("own items per class: ", dict(items), "(val/test are split by item)")

    tf_train, tf_eval = train_transform(args.image_size), eval_transform(args.image_size)
    common = dict(num_workers=args.workers, persistent_workers=args.workers > 0, pin_memory=device.type == "cuda")
    train_loader = DataLoader(Images(train, tf_train), batch_size=args.batch_size,
                              sampler=balanced_sampler(train, args.own_weight), drop_last=True, **common)
    evals = {
        "val_internet": split_i["val"], "test_internet": split_i["test"],
        "val_own": split_o["val"], "test_own": split_o["test"],
    }
    eval_loaders = {k: DataLoader(Images(v, tf_eval), batch_size=64, shuffle=False,
                                  num_workers=min(2, args.workers), pin_memory=device.type == "cuda")
                    for k, v in evals.items() if v}

    model = build_model(args.arch, len(CATEGORIES), image_size=args.image_size).to(device)
    opt = torch.optim.AdamW(param_groups(model, args.arch, args.lr, 0.05 if is_vit or args.arch == "convnext_small" else 0.02,
                                         args.layer_decay))
    steps = args.epochs * len(train_loader)
    warm = len(train_loader) * (2 if is_vit else 1)  # transformers like a longer warmup
    dtype = amp_dtype(device)
    scaler = torch.amp.GradScaler("cuda", enabled=dtype == torch.float16)
    print(f"mixed precision: {dtype}  lr(head)={args.lr}")
    sched = torch.optim.lr_scheduler.LambdaLR(
        opt, lambda s: (s + 1) / warm if s < warm else 0.5 * (1 + math.cos(math.pi * (s - warm) / max(steps - warm, 1))))
    loss_fn = nn.CrossEntropyLoss(label_smoothing=0.1)

    best, best_score, history = None, -1.0, []
    for epoch in range(1, args.epochs + 1):
        model.train()
        t0, tot, n, correct = time.time(), 0.0, 0, 0
        for x, y, _ in train_loader:
            x, y = x.to(device, non_blocking=True), y.to(device, non_blocking=True)
            opt.zero_grad(set_to_none=True)
            with torch.autocast(device.type, dtype=dtype, enabled=dtype is not None):
                out = model(x)
                loss = loss_fn(out, y)
            scaler.scale(loss).backward()
            scaler.unscale_(opt)
            nn.utils.clip_grad_norm_(model.parameters(), 1.0)
            scaler.step(opt)
            scaler.update()
            sched.step()
            tot += loss.item() * len(y)
            n += len(y)
            correct += (out.argmax(1) == y).sum().item()
        rec = {"epoch": epoch, "train_loss": round(tot / n, 4), "train_acc": round(correct / n, 4)}
        for k in ("val_internet", "val_own"):
            if k in eval_loaders:
                lab, pre, _, _ = predict(model, eval_loaders[k], device)
                rec[k + "_macro_f1"] = summarize(lab, pre)["macro_f1"]
        # own photos are what the real bin sees, so they count for half the score when present
        score = rec["val_internet_macro_f1"] if "val_own_macro_f1" not in rec else \
            0.5 * rec["val_internet_macro_f1"] + 0.5 * rec["val_own_macro_f1"]
        rec["score"] = round(score, 4)
        rec["sec"] = round(time.time() - t0)
        history.append(rec)
        print("  ".join(f"{k}={v}" for k, v in rec.items()), flush=True)
        if score > best_score:
            best_score = score
            best = {k: v.detach().cpu().clone() for k, v in model.state_dict().items()}

    model.load_state_dict(best)
    report = {"args": {k: str(v) for k, v in vars(args).items()}, "history": history, "results": {}}
    errors = []
    probs_val, labels_val = [], []
    for k, loader in eval_loaders.items():
        lab, pre, prob, idx = predict(model, loader, device)
        report["results"][k] = summarize(lab, pre)
        if k.startswith("val"):
            probs_val += prob
            labels_val += lab
        for y, p, pr, i in zip(lab, pre, prob, idx):
            if y != p:
                errors.append((k, evals[k][i]["path"], CATEGORIES[y], CATEGORIES[p], round(max(pr), 3)))

    table = threshold_table(labels_val, probs_val)
    chosen = pick_threshold(table, args.max_reject)
    report["threshold_table_val"] = table
    report["threshold"] = chosen
    for k in ("test_internet", "test_own"):
        if k in eval_loaders:
            lab, _, prob, _ = predict(model, eval_loaders[k], device)
            t = [r for r in threshold_table(lab, prob) if r["threshold"] == chosen["threshold"]][0]
            report["results"][k]["with_threshold"] = t

    torch.save({"state_dict": model.state_dict(), "categories": CATEGORIES, "arch": args.arch,
                "image_size": args.image_size, "threshold": chosen["threshold"]}, paths["model"])
    paths["report"].write_text(json.dumps(report, indent=2))
    with paths["errors"].open("w", encoding="utf-8") as f:
        f.write("split\tpath\ttrue\tpred\tconfidence\n")
        for e in sorted(errors, key=lambda e: -e[4]):
            f.write("\t".join(map(str, e)) + "\n")

    print("\n================ RESULTS (best epoch) ================")
    for k, r in report["results"].items():
        print(f"{k:14} n={r['n']:5}  acc={r['accuracy']:.3f}  macroF1={r['macro_f1']:.3f}  "
              f"balanced={r['balanced_accuracy']:.3f}"
              + (f"  | thr {chosen['threshold']}: acc_on_accepted={r['with_threshold']['accuracy_on_accepted']:.3f} "
                 f"supported_rejected={r['with_threshold']['supported_rejected']:.3f} "
                 f"unsupported_caught={r['with_threshold']['unsupported_caught']:.3f}" if "with_threshold" in r else ""))
        print("   " + "  ".join(f"{c[:7]}:R{v['recall']:.2f}/P{v['precision']:.2f}" for c, v in r["per_class"].items()))
    print(f"\nchosen threshold: {chosen}")
    print(f"saved: {paths['model']}, {paths['report']}, {paths['errors']} "
          f"(errors.tsv sorted by confidence: confident mistakes are often wrong labels)")


if __name__ == "__main__":
    main()
