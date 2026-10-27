# Waste classifier

`src/train.py` trains a 5-class image classifier: GENERAL_WASTE, ORGANIC, PAPER, PMD, UNSUPPORTED.

```
python ml/src/train.py --arch convnext_small --batch-size 32
python ml/src/train.py --arch mobilenet_v3_large
```

The dataset folder `../project_dataset_clean` is used by default. Models are saved to `ml/models/model_<Name>.pth`.

## Results

Dataset: 17,125 cleaned internet images, split 80/10/10 with seed 42. Both models are tested on the same
1,712 test images, which were never used for training or for choosing the best epoch. Training ran for 25 epochs.

| Model | File | Size | Test accuracy | Test macro F1 |
|---|---|---|---|---|
| **ConvNeXt-Small** (ImageNet-22k) | `models/model_ConvNeXt-Small.pth` | 198 MB | **0.972** | **0.970** |
| MobileNetV3-Large | `models/model_MobileNetV3-Large_2.pth` | 17 MB | 0.962 | 0.962 |

Test recall per class:

| Model | GENERAL_WASTE | ORGANIC | PAPER | PMD | UNSUPPORTED |
|---|---|---|---|---|---|
| ConvNeXt-Small | 0.96 | 1.00 | 0.96 | 0.96 | 0.98 |
| MobileNetV3-Large | 0.93 | 1.00 | 0.97 | 0.96 | 0.96 |

Each model also stores a confidence threshold. An item is rejected if the model's confidence is below the
threshold or if it predicts UNSUPPORTED.

| Model | Threshold | Correct among accepted | Supported items rejected | Unsupported items caught |
|---|---|---|---|---|
| ConvNeXt-Small | 0.90 | 98.4% | 5.3% | 99.0% |
| MobileNetV3-Large | 0.80 | 99.0% | 9.5% | 99.0% |

ConvNeXt-Small is the best model. MobileNetV3-Large is 12x smaller and about 1% less accurate.
These numbers come from internet images; accuracy on photos taken in the real bin has not been measured yet.
