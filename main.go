package main

/*
@project : basicKVSR
@author  : nomadxzy
@file   : inference_hy.py
@time   : 2023-06-21 22:36:15
*/

import (
	"basicKVSR/sr"
)

func main() {
	sr.CreateDirs([]string{"out/"})

	//inFile := "rtmp://127.0.0.1:1935/live/movie"
	inFile := "in/90p.mp4"
	sr.RunSR(inFile)
}
