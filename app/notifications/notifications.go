package notifications

type INotifier interface {
	LayerChange(newLayer string) error
}

type Disabled struct{}

func (n *Disabled) LayerChange(_ string) error {
	return nil
}
