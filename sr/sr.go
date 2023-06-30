package sr

import (
	"io"
	"log"
	"path/filepath"
	"strings"
)

type Config struct {
	w     int    //原视频宽
	h     int    //原视频高
	scale int    //超分倍数，默认4
	W     int    //新视频宽
	H     int    //新视频高
	srApi string // sr后端服务地址
}

var conf *Config

func RunSR(inFile string) {

	var err error
	initLog()
	w, h := getVideoSize(inFile)
	scale := 4
	conf = &Config{w, h, scale, w * scale, h * scale,
		//"http://localhost:5000/",
		"http://10.112.90.187:5000/",
	}
	log.Println(w, h)

	//pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	InitKeyProcess() //对超分后的图片进行h264编码的服务

	//done1 := transToFlv(inFile, pw1) // 转码为flv
	done2 := FSR(inFile, pw2) //上采样

	_, fileName := filepath.Split(inFile)
	rawName := strings.Split(fileName, ".")[0]
	processKSR(pr2, "out/"+rawName+".flv") // 超分关键帧

	_ = <-done2
	CheckErr(err)
	log.Println("Done")
}
