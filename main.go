// Copyright (c) 2019 Timo Savola. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/tsavola/wag/compile"
	"github.com/tsavola/wag/section"
	"github.com/tsavola/wag/wa"
)

func main() {
	if err := main2(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main2() (err error) {
	var (
		output   string
		set      string
		unset    bool
		unexport bool
	)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] filename\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&output, "o", output, "output filename")
	flag.StringVar(&set, "s", set, "set start function to exported function name")
	flag.BoolVar(&unset, "u", unset, "unset start function")
	flag.BoolVar(&unexport, "x", unexport, "unexport all export functions")
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	input := flag.Arg(0)

	if (output != "") != ((set != "") || unset || unexport) || (set != "" && unset) {
		fmt.Fprintf(flag.CommandLine.Output(), "Inconsistent options\n")
		os.Exit(2)
	}

	in, err := os.Open(input)
	if err != nil {
		return
	}
	defer in.Close()

	sections := section.MakeMap()

	config := compile.ModuleConfig{
		Config: compile.Config{
			SectionMapper: sections.Mapper(),
		},
	}

	m, err := compile.LoadInitialSections(&config, bufio.NewReader(in))
	if err != nil {
		return
	}

	if output == "" {
		if index, defined := m.StartFunc(); defined {
			fmt.Printf("%s: start function index: %d\n", input, index)
		} else {
			fmt.Printf("%s: start function not defined\n", input)
		}
		return
	}

	var startSection []byte

	if set != "" {
		index, sig, found := m.ExportFunc(set)
		if !found {
			err = fmt.Errorf("%s: export function not found: %q", input, set)
			return
		}
		if !sig.Equal(wa.FuncType{}) {
			err = fmt.Errorf("%s: function has unsuitable type: %s%s", input, set, sig)
			return
		}

		startSection = makeStartSection(index)
	}

	out, err := os.Create(output)
	if err != nil {
		return
	}
	defer out.Close()

	_, err = in.Seek(0, io.SeekStart)
	if err != nil {
		return
	}
	if unexport {
		_, err = io.CopyN(out, in, sections.Sections[section.Export].Offset)
		if err != nil {
			return
		}
		_, err = in.Seek(sections.Sections[section.Export].Length, io.SeekCurrent)
		if err != nil {
			return
		}
	} else {
		_, err = io.CopyN(out, in, sections.Sections[section.Start].Offset)
		if err != nil {
			return
		}
	}
	_, err = in.Seek(sections.Sections[section.Start].Length, io.SeekCurrent)
	if err != nil {
		return
	}
	_, err = out.Write(startSection)
	if err != nil {
		return
	}
	_, err = io.Copy(out, in)
	if err != nil {
		return
	}

	return
}

func makeStartSection(index uint32) (buf []byte) {
	indexBuf := make([]byte, binary.MaxVarintLen32)
	indexLen := binary.PutUvarint(indexBuf, uint64(index))

	lengthBuf := make([]byte, binary.MaxVarintLen32)
	lengthLen := binary.PutUvarint(lengthBuf, uint64(indexLen))

	buf = append(buf, byte(section.Start))
	buf = append(buf, lengthBuf[:lengthLen]...)
	buf = append(buf, indexBuf[:indexLen]...)
	return
}
