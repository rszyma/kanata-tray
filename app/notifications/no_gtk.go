//go:build !gtk_overlay

package notifications

import (
	"fmt"
	"time"
)

type GtkOverlay struct{}

func InitGtkOverlay(width int, height int, positionOffsetX int, positionOffsetY int, visibilityDuration time.Duration) (*GtkOverlay, error) {
	return nil, fmt.Errorf("'gtk_overlay' compilation flag was not enabled for this build")
}

func (n *GtkOverlay) LayerChange(newLayer string) error {
	return nil
}
