package greeter

import "fmt"

type Greeter interface {
	Greet(name string) string
}

type greeter struct{}

func (g *greeter) Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

func NewGreeter() Greeter {
	return &greeter{}
}
