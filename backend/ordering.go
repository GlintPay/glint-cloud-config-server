package backend

type Sorter struct {
	Backends Backends
}

func (ss Sorter) Sort() func(i, j int) bool {
	return func(i, j int) bool {
		return ss.Backends[i].Order() < ss.Backends[j].Order()
	}
}
