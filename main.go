package main

/*
@project : basicKVSR
@author  : nomadxzy
@file   : inference_hy.py
@ide    : PyCharm、Goland
@time   : 2023-06-21 22:36:15
*/

import (
	"basicKVSR/sr"
	"fmt"
	"time"
)

func main() {
	sr.CreateDirs([]string{"out/"})

	//inFile := "rtmp://127.0.0.1:1935/live/movie"
	inFile := "in/90p.mp4"
	t1 := time.Now()
	sr.RunSR(inFile)
	t2 := time.Now()
	fmt.Println("time spent:", t2.Sub(t1).Milliseconds())
}
