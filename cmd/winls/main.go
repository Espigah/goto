// cmd/winls: lista as janelas abertas (debug do backend winfocus).
package main

import (
	"fmt"
	"os"

	"goto/internal/winfocus"
)

func main() {
	b, err := winfocus.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
	wins, err := b.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
	fmt.Printf("%d janelas:\n", len(wins))
	for _, w := range wins {
		fmt.Printf("  class=%-22q title=%q\n", w.Class, w.Title)
	}
}
