package main

import (
	"encoding/binary"
	"encoding/json"
	ffmpeg "ffmpeg-go"
	"fmt"
	goflv "github.com/yutopp/go-flv"
	"io"
	"log"
)

// ExampleStream
// inFileName: input filename
// outFileName: output filename
// dream: Use DeepDream frame processing (requires tensorflow)
var flvFile1 *File

func getVideoSize(fileName string) (int, int) {
	log.Println("Getting video size for", fileName)
	data, err := ffmpeg.Probe(fileName)
	if err != nil {
		panic(err)
	}
	log.Println("got video info", data)
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

func startFFmpegProcess1(infileName string, writer io.WriteCloser) <-chan error {
	log.Println("Starting ffmpeg process1")
	done := make(chan error)
	go func() {
		err := ffmpeg.Input(infileName).
			Output("pipe:",
				ffmpeg.KwArgs{
					"vcodec": "libx264", "format": "flv", "pix_fmt": "yuv420p",
				}).
			WithOutput(writer).
			Run()
		log.Println("ffmpeg process1 done")
		_ = writer.Close()
		done <- err
		close(done)
	}()
	return done
}

func startFFmpegProcess2_(reader io.Reader) <-chan error {
	return nil
	done := make(chan error)
	go func() {
		err := ffmpeg.Input("pipe:",
			ffmpeg.KwArgs{"format": "flv"}).
			Output("idr_%03d.png").
			OverWriteOutput().
			WithInput(reader).
			Run()
		checkError(err)
		done <- err
		close(done)
	}()
	return done
}

func startFFmpegProcess3(reader io.Reader) <-chan error {
	log.Println("Starting ffmpeg process3")
	done := make(chan error)
	go func() {

		err := ffmpeg.Input("pipe:",
			ffmpeg.KwArgs{"format": "flv"}).
			Output("fm_sr.flv", ffmpeg.KwArgs{"vf scale": "1440:-1"}).
			OverWriteOutput().
			WithInput(reader).
			Run()
		log.Println("ffmpeg process3 done")
		done <- err
		close(done)
	}()
	return done
}

func startFFmpegProcess2(reader io.Reader) <-chan error {
	log.Println("Starting ffmpeg process2")
	done := make(chan error)

	//flvFile1, _ = CreateProtoFile("t.flv")

	go func() {
		//buf := make([]byte, 1000000)
		//for {
		//	dataRead := make([]byte, 200000)
		//	n, _ := reader.Read(dataRead)
		//	//fmt.Println(string(dataRead[:n]))
		//	fmt.Println("读取了", n)
		//	flvFile1.file.Write(dataRead[:n])
		//}

		err := ffmpeg.Input("pipe:",
			ffmpeg.KwArgs{"format": "flv"}).
			Output("test_out.flv", ffmpeg.KwArgs{"vcodec": "libx264", "format": "flv", "pix_fmt": "yuv420p"}).
			OverWriteOutput().
			WithInput(reader).
			Run()
		log.Println("ffmpeg process2 done")
		done <- err
		close(done)
	}()
	return done
}

func process(reader io.ReadCloser, writer io.WriteCloser) {
	go func() {
		dec, err := goflv.NewDecoder(reader)
		log.Printf("Header: %+v", dec.Header())
		checkErr(err)

		var tmpBuf = make([]byte, 4)
		_, err = io.ReadFull(reader, tmpBuf)
		checkErr(err)

		HEADER_BYTES := []byte{'F', 'L', 'V', 0x01, 0x05, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x00}
		_, _ = writer.Write(HEADER_BYTES)
		flvFile, _ := CreateFile("movie.flv")

		for {
			header, data, err := ReadTag(reader)
			checkErr(err)

			var tag Tag
			_, err = tag.ParseMediaTagHeader(data, header.TagType == byte(9))
			checkErr(err)
			header.pktHeader = &tag

			if header.TagType == byte(9) {

				if vh, ok := header.pktHeader.(VideoPacketHeader); ok {
					fmt.Println(vh.IsKeyFrame(), " ", len(data))

					if vh.IsKeyFrame() || vh.IsSeq() {
						//data := header.TagBytes
						//fmt.Println(data[9])

						err = flvFile.WriteTagDirect(header.TagBytes)
						checkError(err)

						n, err := writer.Write(header.TagBytes)
						if n != len(header.TagBytes) || err != nil {
							panic(fmt.Sprintf("write error: %d, %s", n, err))
						}
						err = binary.Write(writer, binary.BigEndian, uint32(len(data)+11))
						checkErr(err)
					}
				}
			}

		}
	}()
	return
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

type TagHeader struct {
	TagType   byte
	DataSize  uint32
	Timestamp uint32
	pktHeader PacketHeader
	TagBytes  []byte
}

func ReadTag(reader io.ReadCloser) (header *TagHeader, data []byte, err error) {
	tmpBuf := make([]byte, 4)
	header = &TagHeader{}
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

	// Read data
	//lr := io.LimitReader(reader, int64(header.DataSize))
	//switch header.TagType {
	//case byte(9):
	//	var v tag.VideoData
	//	if err := tag.DecodeVideoData(lr, &v); err != nil {
	//		fmt.Printf("failed to decode video data: %v\n", err)
	//	}
	//	break
	//case byte(8):
	//	var a tag.AudioData
	//	if err := tag.DecodeAudioData(lr, &a); err != nil {
	//		fmt.Printf("failed to decode audio data: %v\n", err)
	//	}
	//	break
	//case byte(18):
	//	var s tag.ScriptData
	//	if err := tag.DecodeScriptData(lr, &s); err != nil {
	//		fmt.Printf("failed to decode script data: %v\n", err)
	//	}
	//	break
	//}

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
