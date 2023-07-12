FROM python:3

MAINTAINER zuyunxu@bupt.edu.cn

WORKDIR /home/app

RUN apt update && apt install -y vim libgl1-mesa-glx

COPY realbasicvsr_flask/ ./

RUN pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cpu
RUN pip install --no-cache-dir -r requirements.txt \
    && mim install mmcv-full

EXPOSE 5000

CMD [ "python", "./inference_flask.py" ]