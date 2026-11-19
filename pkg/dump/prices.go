package dump

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

const defaultPricesFile = "./prices.csv"

// PricePoint is one row of the prices file.
type PricePoint struct {
	Date   string
	Price  float64
	Source string
}

// PriceBook maps a symbol to its price points sorted by date.
type PriceBook map[string][]PricePoint

// LoadPriceBook reads a CSV with columns date,symbol,price[,source]. A header
// row and blank or #-comment lines are skipped. A missing file yields an empty
// book so stock plan accounts still dump (with share-denominated income).
func LoadPriceBook(path string) (PriceBook, error) {
	if path == "" {
		path = defaultPricesFile
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return PriceBook{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open prices file: %w", err)
	}
	defer f.Close()
	return parsePriceBook(f)
}

func parsePriceBook(r io.Reader) (PriceBook, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.Comment = '#'
	book := PriceBook{}
	line := 0
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read prices file: %w", err)
		}
		line++
		if len(rec) < 3 {
			continue
		}
		price, err := strconv.ParseFloat(strings.TrimSpace(rec[2]), 64)
		if err != nil {
			if line == 1 {
				continue // header
			}
			return nil, fmt.Errorf("prices file line %d: bad price %q", line, rec[2])
		}
		p := PricePoint{Date: strings.TrimSpace(rec[0]), Price: price}
		if len(rec) > 3 {
			p.Source = strings.TrimSpace(rec[3])
		}
		sym := strings.ToUpper(strings.TrimSpace(rec[1]))
		book[sym] = append(book[sym], p)
	}
	for sym := range book {
		pts := book[sym]
		sort.SliceStable(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
		// a later row for the same date wins (e.g. a Schwab FMV over a close)
		dedup := pts[:0]
		for _, p := range pts {
			if n := len(dedup); n > 0 && dedup[n-1].Date == p.Date {
				dedup[n-1] = p
				continue
			}
			dedup = append(dedup, p)
		}
		book[sym] = dedup
	}
	return book, nil
}

// OnOrBefore returns the latest price at or before date (YYYY-MM-DD).
func (b PriceBook) OnOrBefore(symbol, date string) (PricePoint, bool) {
	pts := b[strings.ToUpper(symbol)]
	i := sort.Search(len(pts), func(i int) bool { return pts[i].Date > date })
	if i == 0 {
		return PricePoint{}, false
	}
	return pts[i-1], true
}
