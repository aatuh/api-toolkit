package swagstub

import (
	"fmt"
	"sync"
	"testing"
)

func TestRegistryConcurrentAccess(t *testing.T) {
	_ = t
	spec := &Spec{InfoInstanceName: "swagger"}
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				Register(fmt.Sprintf("spec-%d-%d", worker, j), spec)
				_ = Get("swagger")
				_ = List()
			}
		}(i)
	}
	wg.Wait()
}
