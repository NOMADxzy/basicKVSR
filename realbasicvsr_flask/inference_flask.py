#!/user/bin/env python
# coding=utf-8
import argparse
import glob
import json
import os
import time

import cv2
import mmcv
import numpy as np
import torch
import uuid
from mmcv.runner import load_checkpoint
from mmedit.core import tensor2img

from realbasicvsr.models.builder import build_model
import ffmpeg

from flask import Flask, request
from PIL import Image


def init_model(config, checkpoint=None):
    if isinstance(config, str):
        config = mmcv.Config.fromfile(config)
    elif not isinstance(config, mmcv.Config):
        raise TypeError('config must be a filename or Config object, '
                        f'but got {type(config)}')
    config.model.pretrained = None
    config.test_cfg.metrics = None
    model = build_model(config.model, test_cfg=config.test_cfg)
    if checkpoint is not None:
        checkpoint = load_checkpoint(model, checkpoint)

    model.cfg = config  # save the config in the model for convenience
    model.eval()

    return model


class Worker:
    def __init__(self):
        self.checkpoint_path = 'checkpoints/RealBasicVSR_x4.pth'
        self.config = 'configs/realbasicvsr_x4.py'
        self.is_save_as_png = True
        self.max_seq_len = 2
        self.model = init_model(self.config, self.checkpoint_path)
        self.frames = 1

    def do_pic(self, img):
        inputs = []
        inputs.append(img)
        for i, img in enumerate(inputs):
            img = torch.from_numpy(img / 255.).permute(2, 0, 1).float()
            inputs[i] = img.unsqueeze(0)
        inputs = torch.stack(inputs, dim=1)
        # map to cuda, if available
        cuda_flag = False
        if torch.cuda.is_available():
            model = self.model.cuda()
            cuda_flag = True
        with torch.no_grad():
            if cuda_flag:
                inputs = inputs.cuda()
            outputs = self.model(inputs, test_mode=True)['output'].cpu()

        for i in range(0, outputs.size(1)):
            output = tensor2img(outputs[:, i, :, :, :])
            mmcv.imwrite(output, '{}/{}.{}'.format("tmp", str(self.frames), "png"))
            self.frames += 1
            return output


worker = Worker()
app = Flask(__name__)


@app.route('/')
def index():
    data = {
        "project": "basicKVSR",
        "author": "nomadxzy",
        "time": "2023-06-21 22:36:15"
    }

    response = json.dumps(data)
    return response, 200, {"Content-Type": "application/json"}


@app.route('/', methods=['POST'])
def sr():
    raw_data = request.get_data()
    w, h = int(request.args.get("w")), int(request.args.get("h"))
    image = Image.frombytes('RGB', (w, h), raw_data)

    img = np.array(image)
    t1 = time.time()
    out_frame = worker.do_pic(img)
    t2 = time.time()
    print("img received: {}x{}".format(w, h), ", sr time spend:", t2 - t1, "s")

    return out_frame.astype(np.uint8).tobytes()


if __name__ == '__main__':

    if not os.path.exists("tmp"):
        os.mkdir("tmp")

    app.run(host="0.0.0.0", port=5000)
