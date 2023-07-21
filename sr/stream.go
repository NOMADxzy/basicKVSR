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

var seqBytes []byte

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

func transToFlv(infileName string, outfileName string) <-chan error {
	Log.Infof("Starting transToFlv")
	done := make(chan error)
	go func() {
		err := ffmpeg.Input(infileName).
			Output(outfileName,
				ffmpeg.KwArgs{
					"vcodec": "copy", "format": "flv", "pix_fmt": "yuv420p",
				}).
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
					"s": fmt.Sprintf("%dx%d", conf.W, conf.H), "format": "flv", "vcodec": "libx264",
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

var buf *bytes.Buffer

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

func readKeyFrame(keyframeBytes []byte, id int) []byte {
	Log.Debugf("Starting read KeyFrame")

	tmpBuf := bytes.NewBuffer(HEADER_BYTES)
	tmpBuf.Write(seqBytes)
	binary.Write(tmpBuf, binary.BigEndian, uint32(len(seqBytes)))
	tmpBuf.Write(keyframeBytes)
	binary.Write(tmpBuf, binary.BigEndian, uint32(len(keyframeBytes)))

	done := clipPreKeyframe(bytes.NewReader(tmpBuf.Bytes()))
	<-done
	if len(buf.Bytes()) == 0 {
		return nil
	}
	body := PostImg(buf.Bytes())

	encToH264(body) //会在keyChan中产生相应的超分tag
	return <-keyChan

}

func parseHeader(header *TagHeader, data []byte) {
	var tag Tag
	_, err := tag.ParseMediaTagHeader(data, header.TagType == byte(9))
	CheckErr(err)
	header.pktHeader = &tag
}

func processKSR(reader_fsr io.ReadCloser, outfile string) *bytes.Buffer {
	//pr, pw := io.Pipe()
	outBuf := bytes.NewBuffer(HEADER_BYTES)
	go func() {
		var tmpBuf = make([]byte, 13) //去除头部字节

		_, err := io.ReadFull(reader_fsr, tmpBuf)
		CheckErr(err)

		flvFile_vsr, _ := CreateFile(outfile)
		//pw.Write(HEADER_BYTES)

		for id := 0; ; id += 1 {
			headerFsr, dataFsr, _ := ReadTag(reader_fsr)

			parseHeader(headerFsr, dataFsr)
			vhFsr, _ := headerFsr.pktHeader.(VideoPacketHeader)

			if headerFsr.TagType == byte(9) {

				if vh, ok := headerFsr.pktHeader.(VideoPacketHeader); ok {
					//err = flvFile.WriteTagDirect(header.TagBytes)
					CheckErr(err)

					if vh.IsSeq() {
						seqBytes = headerFsr.TagBytes

					} else if vh.IsKeyFrame() {
						keyTagBytes := readKeyFrame(headerFsr.TagBytes, id)
						if keyTagBytes != nil {
							err = flvFile_vsr.WriteTagDirect(keyTagBytes)
							CheckErr(err)
							//_, err := pw.Write(keyTagBytes)
							outBuf.Write(keyTagBytes)

							Log.WithFields(logrus.Fields{
								"new_size":    len(keyTagBytes),
								"pre_size":    headerFsr.DataSize + 11,
								"is_KeyFrame": vhFsr.IsKeyFrame(),
							}).Infof("instead keyFrame")
							continue
						}
					}
				}
			}

			err = flvFile_vsr.WriteTagDirect(headerFsr.TagBytes) //非IDR帧数据保持原有
			//_, err := pw.Write(headerFsr.TagBytes)
			outBuf.Write(headerFsr.TagBytes)
			if vhFsr.IsKeyFrame() {
				Log.WithFields(logrus.Fields{
					"size":      headerFsr.DataSize + 11,
					"timestamp": headerFsr.Timestamp,
				}).Warnf("ignore key frame")
				if headerFsr.Timestamp > 0 {
					//pw.Close()
					//pr.Close()
				}
			}
			CheckErr(err)

		}
	}()
	return outBuf
}

func PostImg(bytesData []byte) []byte {

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
