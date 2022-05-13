package pings

import (
	"context"
	"fmt"
	"time"
)

func MonitorWeather(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			h, m, _ := time.Now().Clock()
			if m == 0 && (h == 9 || h == 15) {
				fmt.Printf("sending weather report to user")
			}
		}
	}
}
