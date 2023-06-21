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
	//inFile := "rtmp://127.0.0.1:1935/live/movie"
	inFile := "in/90p.mp4"
	runSR(inFile)
}

func runSR(inFile string) {

	var err error
	initLog()
	w, h := getVideoSize(inFile)
	scale := 4
	conf = &Config{w * scale, h * scale}
	log.Println(w, h)

	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	InitKeyProcess() //对超分后的图片进行h264编码的服务

	done1 := transToFlv(inFile, pw1) // 转码为flv
	done2 := FSR(inFile, pw2)

	_, fileName := filepath.Split(inFile)
	rawName := strings.Split(fileName, ".")[0]
	processKSR(pr1, pr2, "out/"+rawName+".flv") // 提取关键帧

	err = <-done1
	checkErr(err)
	_ = <-done2
	checkErr(err)
	log.Println("Done")
}
