package main

import (
	"fmt"

	"github.com/mickamy/injector/example/returns/greeter"
)

//injector:container name=NewGreeter
type app struct {
	service greeter.Greeter `inject:"returns"`
}

func (a *app) Greet(name string) string {
	return a.service.Greet(name)
}

func main() {
	g := NewGreeter()
	fmt.Println(g.Greet("World"))
}
