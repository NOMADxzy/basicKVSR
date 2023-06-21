package sr

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"io"
)

var pr *io.PipeReader
var pw *io.PipeWriter
var keyChan chan []byte

func encToH264(buf []byte) {
	Log.Debugf("starting encode h264")
	_ = ffmpeg.Input("pipe:",
		ffmpeg.KwArgs{"format": "rawvideo", "pix_fmt": "bgr24", "s": fmt.Sprintf("%dx%d", conf.w, conf.h)}).
		Output("pipe:", ffmpeg.KwArgs{"pix_fmt": "yuv420p", "vcodec": "libx264", "format": "flv", "r": 30, "vframes": 1}).
		OverWriteOutput().
		WithInput(bytes.NewReader(buf)).
		WithOutput(pw).
		Run()
	Log.Debugf("encode h264 done")
}

func clipKeyframe(reader io.ReadCloser, keyChan chan []byte) {
	Log.Infof("waiting for flv of keyframe")

	go func() {

		for i := 0; ; i++ {
			i = i % 4
			if i == 0 {
				var tmpBuf = make([]byte, 13) //去除头部字节
				_, _ = io.ReadFull(reader, tmpBuf)
			}

			header, _, err := ReadTag(reader)
			CheckErr(err)
			if header.TagType == byte(9) && header.DataSize > 100 {
				Log.WithFields(logrus.Fields{
					"size": header.DataSize + 11,
				}).Debugf("截取关键帧")
				keyChan <- header.TagBytes
			}

		}
	}()
}

func InitKeyProcess() {
	Log.Infof("初始化h264编码抽帧模块")

	pr, pw = io.Pipe()
	keyChan = make(chan []byte, 1)
	clipKeyframe(pr, keyChan) //等待编码好的关键帧视频包
}
