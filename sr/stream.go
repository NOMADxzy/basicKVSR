package main

import (
	"encoding/binary"
	"encoding/json"
	ffmpeg "ffmpeg-go"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
)

// ExampleStream
// inFileName: input filename
// outFileName: output filename
// dream: Use DeepDream frame processing (requires tensorflow)
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

func transToFlv(infileName string, writer io.WriteCloser) <-chan error {
	Log.Infof("Starting transToFlv")
	done := make(chan error)
	go func() {
		err := ffmpeg.Input(infileName).
			Output("pipe:",
				ffmpeg.KwArgs{
					"vcodec": "copy", "format": "flv", "pix_fmt": "yuv420p",
				}).
			WithOutput(writer).
			Run()
		Log.Infof("transToFlv done")
		_ = writer.Close()
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
					"s": fmt.Sprintf("%dx%d", conf.w, conf.h), "format": "flv", "vcodec": "libx264",
				}).
			WithOutput(writer).
			Run()
		Log.Infof("ffmpeg sr done")
		//_ = writer.Close()
		done <- err
		close(done)
	}()

	return done
}

func readKeyFrame(keyframeBytes []byte, id int) []byte {
	Log.Debugf("Starting read KeyFrame")

	tmpFile, _ := CreateProtoFile(fmt.Sprintf("tmp/%d.flv", id))
	tmpFile.file.Write(HEADER_BYTES)
	tmpFile.file.Write(seqBytes)
	binary.Write(tmpFile.file, binary.BigEndian, uint32(len(seqBytes)))
	tmpFile.file.Write(keyframeBytes)
	binary.Write(tmpFile.file, binary.BigEndian, uint32(len(keyframeBytes)))
	tmpFile.Close()

	command := exec.Command("ffmpeg", "-y", "-i", fmt.Sprintf("tmp/%d.flv", id), fmt.Sprintf("tmp/%d.png", id))
	err := command.Run()
	checkErr(err)

	os.Remove(fmt.Sprintf("tmp/%d.flv", id)) //移除无用文件

	img_path := fmt.Sprintf("/Users/nomad/Desktop/ffmpeg-go/tmp/%d.png", id)
	resp, err := http.Get("http://127.0.0.1:5000/?img_path=" + img_path)
	checkErr(err)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	encToH264(body) //会在keyChan中产生相应的超分tag
	return <-keyChan

}

func parseHeader(header *TagHeader, data []byte) {
	var tag Tag
	_, err := tag.ParseMediaTagHeader(data, header.TagType == byte(9))
	checkErr(err)
	header.pktHeader = &tag
}

func processKSR(reader io.ReadCloser, reader_fsr io.ReadCloser, outfile string) {
	go func() {

		var tmpBuf = make([]byte, 13) //去除头部字节
		_, err := io.ReadFull(reader, tmpBuf)
		checkErr(err)
		_, _ = io.ReadFull(reader_fsr, tmpBuf)

		//flvFile, _ := CreateFile("movie.flv")
		flvFile_vsr, _ := CreateFile(outfile)

		for id := 0; ; id += 1 {
			header, data, _ := ReadTag(reader)
			header_fsr, data_fsr, _ := ReadTag(reader_fsr)

			parseHeader(header, data)
			parseHeader(header_fsr, data_fsr)
			vh_fsr, _ := header_fsr.pktHeader.(VideoPacketHeader)

			if header.TagType == byte(9) {

				if vh, ok := header.pktHeader.(VideoPacketHeader); ok {
					//err = flvFile.WriteTagDirect(header.TagBytes)
					checkErr(err)

					if vh.IsSeq() {
						seqBytes = header.TagBytes

					} else if vh.IsKeyFrame() {
						keyTagBytes := readKeyFrame(header.TagBytes, id)
						Log.WithFields(logrus.Fields{
							"new_size":    len(keyTagBytes),
							"pre_size":    header_fsr.DataSize + 11,
							"is_KeyFrame": vh_fsr.IsKeyFrame(),
						}).Infof("instead keyFrame")
						err = flvFile_vsr.WriteTagDirect(keyTagBytes)
						checkErr(err)
						continue
					}
				}
			}

			err = flvFile_vsr.WriteTagDirect(header_fsr.TagBytes) //非IDR帧数据保持原有
			if vh_fsr.IsKeyFrame() {
				Log.WithFields(logrus.Fields{
					"size":      header_fsr.DataSize + 11,
					"timestamp": header_fsr.Timestamp,
				}).Warnf("ignore key frame")
			}
			checkErr(err)

		}
	}()
	return
}
