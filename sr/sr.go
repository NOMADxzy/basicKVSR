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
	w, h := getVideoSize(inFile)
	log.Println(w, h)

	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	//pr3, pw3 := io.Pipe()
	done1 := startFFmpegProcess1(inFile, pw1) // 转码为flv
	process(pr1, pw2)                         // 提取关键帧
	done2 := startFFmpegProcess2(pr2)         // 解码
	//done3 := startFFmpegProcess3(pr3)
	err := <-done1
	checkErr(err)
	if err != nil {
		panic(err)
	}
	err = <-done2
	checkErr(err)
	log.Println("Done")
}
