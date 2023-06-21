#!/user/bin/env python
# coding=utf-8
import argparse
import glob
import os

import cv2
import mmcv
import numpy as np
import torch
import uuid
from mmcv.runner import load_checkpoint
from mmedit.core import tensor2img

from realbasicvsr.models.builder import build_model
import ffmpeg

from flask import Flask,request


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

    def do_pic(self, input_image_path: str):
        inputs = []
        img = mmcv.imread(input_image_path, channel_order='rgb')
        ext = os.path.basename(input_image_path).split('.')[-1]
        inputs.append(img)
        # inputs.append(img)
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
            filename = '{}.{}'.format(uuid.uuid1().hex, ext)
            if self.is_save_as_png:
                file_extension = os.path.splitext(filename)[1]
            mmcv.imwrite(output, input_image_path)
            return output

process2 = (
        ffmpeg
        .input('pipe:', format='rawvideo', pix_fmt='bgr24', s='{}x{}'.format(1280, 720))
        .output('pipe:', format='flv', pix_fmt='yuv420p',r=30, vframes=1)
        .overwrite_output()
        .run_async(pipe_stdin=True)
    )

worker = Worker()
app = Flask(__name__)
@app.route('/')
def index():
    img_path = request.args["img_path"]
    print(img_path)
    out_frame = worker.do_pic(img_path)
    return out_frame.astype(np.uint8).tobytes()


if __name__ == '__main__':

    # out_frame = worker.do_pic('2.png')
    # outframe = out_frame.astype(np.uint8).tobytes()

    app.run()


    # out,_ = process2.stdin.write(
    #     out_frame
    #     .astype(np.uint8)
    #     .tobytes()
    # )

    # out, _ = (
    #     ffmpeg
    #     .input('2.png', r=1)
    #     .output('pipe:', format='h264', pix_fmt='yuv420p',r=30, vframes=1)
    #     .run(capture_stdout=True)
    # )
    # with open("2.flv", "wb") as f:
    #     f.write(out)

