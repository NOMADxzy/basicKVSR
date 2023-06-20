package main

import (
	"io"
	"log"
)

func main() {
	inFile := "rtmp://127.0.0.1:1935/live/movie"
	runSR(inFile)
}

func runSR(inFile string) {
	video := "gua_180p.mp4"
	video_fsr := "gua_fsr.mp4"

	var err error
	w, h := getVideoSize(video)
	log.Println(w, h)

	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	InitKeyProcess()

	done1 := transToFlv(video, pw1) // 转码为flv
	_ = transToFlv(video_fsr, pw2)
	process(pr1, pr2) // 提取关键帧
	//done2 := startFFmpegProcess2(pr2)         // 解码

	err = <-done1
	checkErr(err)
	if err != nil {
		panic(err)
	}
	log.Println("Done")
}
