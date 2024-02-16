//go:build !no_gtk_overlay

package notifications

import (
	"fmt"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

type GtkOverlay struct {
	windowVisibilityDuration time.Duration

	label                           *gtk.Label
	refreshWindowVisibilityDuration chan time.Time
}

func InitGtkOverlay(width int, height int, positionOffsetX int, positionOffsetY int, visibilityDuration time.Duration) (*GtkOverlay, error) {
	gtk.Init(nil)

	// Create a new window
	win, err := gtk.WindowNew(gtk.WINDOW_POPUP)
	if err != nil {
		return nil, fmt.Errorf("unable to create window: %v", err)
	}
	win.SetDefaultSize(width, height)
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})
	win.SetAcceptFocus(false) // not respected in Hyprland
	win.SetCanFocus(false)    // not respected in Hyprland
	win.SetDecorated(false)
	win.SetTypeHint(gdk.WINDOW_TYPE_HINT_NOTIFICATION)
	win.SetDeletable(false) // not respected in Hyprland
	win.SetSkipTaskbarHint(false)
	win.SetSkipPagerHint(false)
	win.SetResizable(false) // not respected in Hyprland
	win.SetFocusOnMap(false)
	win.SetFocusOnClick(false)
	win.SetCanDefault(false)
	// win.SetOpacity(0.5) // not respected in Hyprland

	win.SetPosition(gtk.WIN_POS_CENTER) // temporary position
	centerX, centerY := win.GetPosition()

	// Create a label for displaying the layer name
	label, err := gtk.LabelNew("Layer Switched!")
	if err != nil {
		return nil, fmt.Errorf("unable to create label: %v", err)
	}

	// Add the label to the window
	win.Add(label)

	refreshVisibilityCh := make(chan time.Time)

	go func() {
		for {
			hideTime := <-refreshVisibilityCh
			timer := time.NewTimer(time.Until(hideTime))

			// needs to be set every time window gets hidden, otherwise it's not respected by gtk
			win.Move(centerX+positionOffsetX, centerY+positionOffsetY)
			win.ShowAll()

		loop:
			for {
				select {
				case newHideTime := <-refreshVisibilityCh:
					if !timer.Stop() {
						<-timer.C
					}
					timer.Stop()
					timer.Reset(time.Until(newHideTime))
				case <-timer.C:
					break loop
				}
			}

			win.Hide()
		}
	}()

	// Start the GTK main loop
	go gtk.Main()

	return &GtkOverlay{
		windowVisibilityDuration:        visibilityDuration,
		label:                           label,
		refreshWindowVisibilityDuration: refreshVisibilityCh,
	}, nil
}

func (n *GtkOverlay) LayerChange(newLayer string) error {
	n.label.SetText(newLayer)
	n.refreshWindowVisibilityDuration <- time.Now().Add(n.windowVisibilityDuration)
	return nil
}
