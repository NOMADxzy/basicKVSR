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
    GopSize int  // 固定gop序列长度
}

var conf *Config

func RunSR(inFile string) {

	var err error
	initLog()
	w, h := getVideoSize(inFile)
	scale := 4
	conf = &Config{w, h, scale, w * scale, h * scale,
		"http://localhost:5001/",
		//"http://10.112.90.187:5001/",
		//"http://10.112.55.254:5001",
		10, // 最大GOP间隔，决定keyframe频率
	}

	pr2, pw2 := io.Pipe()
	InitKeyProcess() //对超分后的图片进行h264编码的服务

	//done1 := transToFlv(inFile, pw1) // 转码为flv
	fsrDone := FSR(inFile, pw2) //上采样

	_, fileName := filepath.Split(inFile)
	rawName := strings.Split(fileName, ".")[0]

	_ = processKSR(pr2, "out/"+rawName+".flv") // 对上采样后的视频中关键帧下采样再超分，其他帧保持不变，即可正确解码

	_ = <-fsrDone
	CheckErr(err)

	//transDone := transToFlv("out/"+rawName+".flv", "out/"+rawName+"s.flv")
	//_ = <-transDone
	log.Println("All Done")
}
