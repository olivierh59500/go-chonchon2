// Command extract recovers the original executable for asset extraction.
package main

import (
	"flag"
	"github.com/olivierh59500/go-chonchon2/internal/source"
	"log"
	"os"
)

func main() {
	input := flag.String("input", "", "packed executable")
	output := flag.String("output", "", "unpacked executable")
	flag.Parse()
	if *input == "" || *output == "" {
		log.Fatal("-input and either -output or -assets required")
	}
	b, e := os.ReadFile(*input)
	if e != nil {
		log.Fatal(e)
	}
	b, e = source.UnpackPRG(b)
	if e != nil {
		log.Fatal(e)
	}
	if *output != "" {
		if e = os.WriteFile(*output, b, 0644); e != nil {
			log.Fatal(e)
		}
	}
	log.Printf("Recovered %d bytes", len(b))
}
