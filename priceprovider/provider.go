package priceprovider

type Provider interface {
	// GetPrices returns latest prices for the given symbols.
	GetPrices(symbols []string) (map[string]float64, error)
}

