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

	mode := flag.Int("m", 16, "Decode Mode Number")
	loop := flag.Int("l", 0, "Loop Number")
	volume := flag.Float64("v", 1.0, "Volume")

	chipKey1 := flag.Uint("c1", 0x30DBE1A8, "Cipher Key 1")
	chipKey2 := flag.Uint("c2", 0xCC554639, "Cipher Key 2")

	h := hca.NewDecoder()
	h.CiphKey1 = uint32(*chipKey1)
	h.CiphKey2 = uint32(*chipKey2)
	h.Mode = *mode
	h.Loop = *loop
	h.Volume = float32(*volume)

	flag.Parse()
	files := flag.Args()

	for _, filename := range files {
		name := filepath.Base(filename)

		setSaveDir := (*saveDir) != defaultDir
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
