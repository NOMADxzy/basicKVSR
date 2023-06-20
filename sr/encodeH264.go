package main

import (
	"bytes"
	ffmpeg "ffmpeg-go"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

func encToH264(out io.Writer, buf io.Reader, width, height int) {
	log.Println("starting encode h264")
	_ = ffmpeg.Input("pipe:",
		ffmpeg.KwArgs{"format": "rawvideo", "pix_fmt": "bgr24", "s": fmt.Sprintf("%dx%d", width, height)}).
		Output("pipe:", ffmpeg.KwArgs{"pix_fmt": "yuv420p", "vcodec": "libx264", "format": "flv", "r": 30, "vframes": 1}).
		OverWriteOutput().
		WithInput(buf).
		WithOutput(out).
		Run()
	fmt.Println("encode h264 done")
}

func clipKeyframe(reader io.ReadCloser, keyChan chan []byte) {
	log.Println("Starting clip keyframe")

	go func() {
		var tmpBuf = make([]byte, 13) //去除头部字节
		_, _ = io.ReadFull(reader, tmpBuf)

		for i := 0; ; i++ {
			header, _, err := ReadT(reader)
			checkerr(err)
			if header.TagType == byte(9) && header.DataSize > 100 {
				fmt.Println("截取关键帧大小=", header.DataSize+11)
				keyChan <- header.TagBytes
			}

		}
	}()
}

func main() {
	resp, err := http.Get("http://127.0.0.1:5000/?img_path=/Users/nomad/Desktop/ffmpeg-go/tmp/2.png")
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	pr, pw := io.Pipe()
	keyChan := make(chan []byte, 1)
	clipKeyframe(pr, keyChan) //等待编码好的关键帧视频包

	encToH264(pw, bytes.NewReader(body), 1280, 720)
	kF := <-keyChan
	fmt.Println(len(kF))
}

func checkerr(err error) {
	if err != nil {
		panic(err)
	}
}

type TagHead struct {
	TagType   byte
	DataSize  uint32
	Timestamp uint32
	pktHeader PacketHeader
	TagBytes  []byte
}

func ReadT(reader io.ReadCloser) (header *TagHead, data []byte, err error) {
	tmpBuf := make([]byte, 4)
	header = &TagHead{}
	// Read tag header
	if _, err = io.ReadFull(reader, tmpBuf[3:]); err != nil {
		return
	}
	header.TagType = tmpBuf[3]

	// Read data size
	if _, err = io.ReadFull(reader, tmpBuf[1:]); err != nil {
		return
	}
	header.DataSize = uint32(tmpBuf[1])<<16 | uint32(tmpBuf[2])<<8 | uint32(tmpBuf[3])
	tagBuf := make([]byte, 11+header.DataSize)
	tagBuf[0] = header.TagType
	copy(tagBuf[1:4], tmpBuf[1:])

	// Read timestamp
	if _, err = io.ReadFull(reader, tmpBuf); err != nil {
		return
	}
	header.Timestamp = uint32(tmpBuf[3])<<24 + uint32(tmpBuf[0])<<16 + uint32(tmpBuf[1])<<8 + uint32(tmpBuf[2])
	copy(tagBuf[4:8], tmpBuf)

	// Read stream ID
	if _, err = io.ReadFull(reader, tmpBuf[1:]); err != nil {
		return
	}
	copy(tagBuf[8:], tmpBuf[1:])

	data = make([]byte, header.DataSize)
	if _, err = io.ReadFull(reader, data); err != nil {
		return
	}
	copy(tagBuf[11:], data)
	header.TagBytes = tagBuf

	// Read previous tag size
	if _, err = io.ReadFull(reader, tmpBuf); err != nil {
		return
	}

	return
}
