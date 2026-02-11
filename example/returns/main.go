package main

import (
	"fmt"

	"github.com/mickamy/injector/example/returns/greeter"
)

type app struct {
	_       greeter.Greeter `inject:"returns"`
	service greeter.Greeter `inject:""`
}

func (a *app) Greet(name string) string {
	return a.service.Greet(name)
}

func main() {
	g := NewGreeter()
	fmt.Println(g.Greet("World"))
}
