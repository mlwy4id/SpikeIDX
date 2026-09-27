package idx

import (
	"context"
	"errors"

	"spikeidx/internal/domain"
)

type Client struct{}

func New() *Client { return &Client{} }

func (c *Client) Search(_ context.Context, _ string) ([]domain.Stock, error) {
	return nil, errors.New("idx: search not implemented in v1 (use yahoo)")
}

func (c *Client) DailyOHLCV(_ context.Context, _ domain.Code) ([]domain.Candle, error) {
	return nil, errors.New("idx: fallback not implemented yet (see architecture.md)")
}
