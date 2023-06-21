package main

import (
	"basicKVSR/sr"
	"os"
)

func checkDirs(dirs []string) {
	for _, dir := range dirs {
		if exist, _ := sr.PathExists(dir); !exist { //文件夹不存在
			err := os.Mkdir(dir, 0755)
			sr.CheckErr(err)
		}
	}
}

func main() {
	checkDirs([]string{"in/", "out/", "tmp/"})

	//inFile := "rtmp://127.0.0.1:1935/live/movie"
	inFile := "in/gua_180p.mp4"
	sr.RunSR(inFile)
}
