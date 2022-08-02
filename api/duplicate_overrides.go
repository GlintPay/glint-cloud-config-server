package api

import "fmt"

//Represent an explicit override that has no effect and can be removed
type duplicate struct {
	key    string
	value  interface{}
	source string
}

func (d duplicate) String() string {
	return fmt.Sprintf("%s: %s (%s);", d.key, d.value, d.source)
}
