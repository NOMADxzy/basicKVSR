package main

import (
	"io"
	"log"
	"path/filepath"
	"strings"
)

type Config struct {
	w int
	h int
}

var conf *Config

func main() {
	inFile := "rtmp://127.0.0.1:1935/live/movie"
	runSR(inFile)
}

func runSR(inFile string) {

	var err error
	w, h := getVideoSize(inFile)
	scale := 4
	conf = &Config{w * scale, h * scale}
	log.Println(w, h)

	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	InitKeyProcess()

	done1 := transToFlv(inFile, pw1) // 转码为flv
	done2 := FSR(inFile, pw2)

	_, fileName := filepath.Split(inFile)
	rawName := strings.Split(fileName, ".")[0]
	processKSR(pr1, pr2, "out/"+rawName+".flv") // 提取关键帧
	//done2 := startFFmpegProcess2(pr2)         // 解码

	err = <-done1
	checkErr(err)
	_ = <-done2
	log.Println("Done")
}
