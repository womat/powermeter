package energy

type Meter interface {
	Listen(string) error
	AddMeasurand(measurand map[string]string)
	ListMeasurand() []string
	GetMeteredValue(string) (float64, error)
	Close() error
	String() string
}
