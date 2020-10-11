package energy

type Meter interface {
	GetEnergyCounter() (float64, error)
	Listen(string) error
	Close() error
}
