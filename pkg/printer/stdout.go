package printer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

var _ Printer = (*Stdout)(nil)

// Stdout implements [Printer] for standard output.
type Stdout struct{}

// Write implements [Printer].
func (s *Stdout) Write(data ...any) error {
	_, err := fmt.Print(data...)
	return err
}

// PriceWriter implements [Printer].
func (s *Stdout) PriceWriter(ctx context.Context, readCh <-chan TopLevelPrices) {
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-readCh:
			if !ok {
				return
			}
			if out, err := json.Marshal(data); err == nil {
				if err = s.Write(fmt.Sprintf("%s\n", out)); err != nil {
					log.Printf("could not write to stdout: %v\n", err)
				}
			}
		}
	}
}
