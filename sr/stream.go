package sr

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"io"
	"io/ioutil"
	"net/http"
)

var seqBytes []byte   // 视频的解码参数信息
var buf *bytes.Buffer // 图像缓存

func getVideoSize(fileName string) (int, int) {
	Log.Infof("Getting video size for", fileName)
	data, err := ffmpeg.Probe(fileName)
	if err != nil {
		panic(err)
	}
	//log.Println("got video info", data)
	type VideoInfo struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int
			Height    int
		} `json:"streams"`
	}
	vInfo := &VideoInfo{}
	err = json.Unmarshal([]byte(data), vInfo)
	if err != nil {
		panic(err)
	}
	for _, s := range vInfo.Streams {
		if s.CodecType == "video" {
			return s.Width, s.Height
		}
	}
	return 0, 0
}

func transToFlv(pr io.ReadCloser, outfile string) <-chan error {
	Log.Infof("Starting transToFlv")
	done := make(chan error)
	go func() {
		err := ffmpeg.Input("pipe:").
			Output(outfile,
				ffmpeg.KwArgs{
					"vcodec": "copy", "format": "flv", "pix_fmt": "yuv420p",
				}).
			WithInput(pr).
			OverWriteOutput().
			Run()
		Log.Infof("transToFlv done")
		done <- err
		close(done)
	}()
	return done
}

func FSR(infileName string, writer io.WriteCloser) <-chan error {
	Log.Infof("Starting ffmpeg sr")
	done := make(chan error)
	go func() {
		err := ffmpeg.Input(infileName).
			Output("pipe:",
				ffmpeg.KwArgs{
					"s": fmt.Sprintf("%dx%d", conf.W, conf.H),
					"g":          conf.GopSize,
					 "format": "flv",
					 "vcodec": "libx264",
				}).
			WithOutput(writer).
			Run()
		Log.Infof("ffmpeg fsr done")
		//_ = writer.Close()
		done <- err
		close(done)
	}()

	return done
}

func clipPreKeyframe(reader io.Reader) chan error {

	buf = bytes.NewBuffer(nil)
	done := make(chan error)
	go func() {
		err := ffmpeg.Input("pipe:",
			ffmpeg.KwArgs{"format": "flv"}).
			Output("pipe:", ffmpeg.KwArgs{"format": "rawvideo", "s": fmt.Sprintf("%dx%d", conf.w, conf.h), "pix_fmt": "rgb24"}).
			WithInput(reader).
			WithOutput(buf).
			Run()
		done <- err
		close(done)
	}()
	return done
}

func constructSingleFlv(keyframeBytes []byte) []byte {
	tmpBuf := bytes.NewBuffer(HEADER_BYTES)
	tmpBuf.Write(seqBytes)
	binary.Write(tmpBuf, binary.BigEndian, uint32(len(seqBytes)))
	tmpBuf.Write(keyframeBytes)
	binary.Write(tmpBuf, binary.BigEndian, uint32(len(keyframeBytes)))
	return tmpBuf.Bytes()
}

func readKeyFrame(keyframeBytes []byte, id int) []byte {
	Log.Debugf("Starting read KeyFrame")

	singleFlvBytes := constructSingleFlv(keyframeBytes)

	done := clipPreKeyframe(bytes.NewReader(singleFlvBytes)) //调用ffmpeg解码出图像
	<-done
	if len(buf.Bytes()) > 0 {
		body := PostImg(buf.Bytes()) //耗时操作1

		encToH264(body) //会在keyChan中产生相应的超分tag
		return <-keyChan
	} else {
		return nil
	}
}

func parseHeader(header *TagHeader, data []byte) {
	var tag Tag
	_, err := tag.ParseMediaTagHeader(data, header.TagType == byte(9))
	CheckErr(err)
	header.pktHeader = &tag
}

func processKSR(readerFsr io.ReadCloser, outfile string) <-chan error { //flv流的方式读入视频，对关键帧先降分辨率再超分
	pr, pw := io.Pipe()
	TransDone := transToFlv(pr, outfile) //将重组后的flv流再转码一次，否则大部分播放器无法正确播放

	go func() {
		var tmpBuf = make([]byte, 13) //去除头部字节

		_, err := io.ReadFull(readerFsr, tmpBuf)
		CheckErr(err)

		//flvFile_vsr, _ := CreateFile(outfile)
		_, err = pw.Write(HEADER_BYTES)
		CheckErr(err)

		for id := 0; ; id += 1 {
			headerFsr, dataFsr, _ := ReadTag(readerFsr)

			parseHeader(headerFsr, dataFsr)
			vhFsr, _ := headerFsr.pktHeader.(VideoPacketHeader)

			if headerFsr.TagType == byte(9) {

				if vh, ok := headerFsr.pktHeader.(VideoPacketHeader); ok {
					CheckErr(err)

					if vh.IsSeq() {
						seqBytes = headerFsr.TagBytes

					} else if vh.IsKeyFrame() { //耗时操作2，需要等到关键帧超分完插入后再读取后面的非关键帧直到下一个关键帧
						keyTagBytes := readKeyFrame(headerFsr.TagBytes, id)
						if keyTagBytes != nil {
							//err = flvFile_vsr.WriteTagDirect(keyTagBytes)
							CheckErr(err)
							_, err := pw.Write(keyTagBytes)
							CheckErr(err)
							err = binary.Write(pw, binary.BigEndian, uint32(len(keyTagBytes)))
							CheckErr(err)

							Log.WithFields(logrus.Fields{
							    "tag_id":      id,
								"new_size":    len(keyTagBytes),
								"pre_size":    headerFsr.DataSize + 11,
								"is_KeyFrame": vhFsr.IsKeyFrame(),
							}).Infof("instead keyFrame")
							continue
						}
					}
				}
			}

			//err = flvFile_vsr.WriteTagDirect(headerFsr.TagBytes)
			_, err := pw.Write(headerFsr.TagBytes) //非IDR帧数据保持原有
			CheckErr(err)
			err = binary.Write(pw, binary.BigEndian, uint32(len(headerFsr.TagBytes)))
			CheckErr(err)
			if vhFsr.IsKeyFrame() {
				Log.WithFields(logrus.Fields{
					"size":      headerFsr.DataSize + 11,
					"timestamp": headerFsr.Timestamp,
				}).Warnf("ignore key frame")

			}
			CheckErr(err)

		}
	}()
	return TransDone
}

func PostImg(bytesData []byte) []byte { //调用后端超分

	request, err := http.NewRequest("POST", fmt.Sprintf("%s?w=%d&h=%d", conf.srApi, conf.w, conf.h), bytes.NewBuffer(bytesData))
	CheckErr(err)
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")

	client := http.Client{}
	resp, err := client.Do(request)
	CheckErr(err)

	body, err := ioutil.ReadAll(resp.Body)
	CheckErr(err)

	return body
}
