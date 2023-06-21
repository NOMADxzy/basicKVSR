package main

import (
	"github.com/sirupsen/logrus"
	"io"
	"os"
)

var Log *logrus.Logger

func initLog() {
	Log = logrus.New()
	Log.Formatter = new(logrus.TextFormatter) //初始化log
	Log.Level = logrus.TraceLevel
	Log.Out = os.Stdout
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

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
