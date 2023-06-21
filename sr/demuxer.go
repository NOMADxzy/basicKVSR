package sr

import (
	"fmt"
)

var (
	ErrAvcEndSEQ = fmt.Errorf("avc end sequence")
)

func DemuxHeader(p *Packet) error {
	var tag Tag
	_, err := tag.ParseMediaTagHeader(p.Data[11:], p.IsVideo)
	if err != nil {
		return err
	}
	p.Header = &tag

	return nil
}

func Demux(p *Packet) error {
	var tag Tag
	n, err := tag.ParseMediaTagHeader(p.Data, p.IsVideo)
	if err != nil {
		return err
	}
	if tag.CodecID() == VIDEO_H264 &&
		p.Data[0] == 0x17 && p.Data[1] == 0x02 {
		return ErrAvcEndSEQ
	}
	p.Header = &tag
	p.Data = p.Data[n:]

	return nil
}
