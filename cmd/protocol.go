package cmd

import (
	"encoding/binary"
	"io"
)

type request struct {
	seq uint32
}

func (req *request) read(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &req.seq); err != nil {
		return err
	}

	return nil
}

func (req *request) write(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, req.seq); err != nil {
		return err
	}

	return nil
}

type response struct {
	req     uint32
	tsNano  int64
	txBytes uint64
	rxBytes uint64
}

func (res *response) read(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &res.req); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &res.tsNano); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &res.txBytes); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &res.rxBytes); err != nil {
		return err
	}

	return nil
}

func (res *response) write(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, res.req); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, res.tsNano); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, res.txBytes); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, res.rxBytes); err != nil {
		return err
	}

	return nil
}
