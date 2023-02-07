package main

import (
	"fmt"
	hypercache "github.com/webscalesoftwareltd/hypercache/clients/golang"
	"time"
)

func main() {
	impls := make([]hypercache.HNPImplementation, 150)
	for i := 0; i < 150; i++ {
		var err error
		impls[i], err = hypercache.NewConnectionWithHNPAddr(
			"127.0.0.1:6060", "", 0)
		if err != nil {
			panic(err)
		}
	}
	fmt.Println("connected!")
	count := 100000

	var total time.Duration
	for i := 0; i < count; i++ {
		t1 := time.Now()
		_, _ = impls[0].Get([]byte("hello"))
		t2 := time.Now()
		total += t2.Sub(t1)
	}
	average := total / time.Duration(count)
	fmt.Println("Average:", average)
}
