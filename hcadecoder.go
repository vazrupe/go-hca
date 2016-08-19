package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/vazrupe/go-hca/hca"
)

func main() {
	defaultDir := "\x00"
	saveDir := flag.String("save", defaultDir, "Save Directory, No Option: hva directory")

	ciphKey1 := flag.Uint("c1", 0x30DBE1A8, "Cipher Key 1")
	ciphKey2 := flag.Uint("c2", 0xCC554639, "Cipher Key 2")

	mode := flag.Int("m", 16, "Decode Mode Number")
	loop := flag.Int("l", 0, "Loop Number")
	volume := flag.Float64("v", 1.0, "Volume")

	flag.Parse()
	files := flag.Args()

	h := hca.NewDecoder()
	h.CiphKey1 = uint32(*ciphKey1)
	h.CiphKey2 = uint32(*ciphKey2)
	h.Mode = *mode
	h.Loop = *loop
	h.Volume = float32(*volume)

	for _, filename := range files {
		setSaveDir := (*saveDir) != defaultDir
		ext := filepath.Ext(filename)
		savename := filename[:len(filename)-len(ext)] + ".wav"
		if setSaveDir {
			_, name := filepath.Split(savename)
			savename = filepath.Join(*saveDir, name)
		}

		if h.DecodeFromFile(filename, savename) {
			fmt.Printf("Decode: %s -> %s\n", filename, savename)
		} else {
			fmt.Printf("Failed: %s\n", filename)
		}
	}
}
