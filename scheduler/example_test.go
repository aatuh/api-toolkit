package scheduler_test

import (
	"context"
	"fmt"
	"time"

	"github.com/aatuh/api-toolkit/v3/scheduler"
)

func ExampleRecorderFunc() {
	var recorded string
	recorder := scheduler.RecorderFunc(func(_ context.Context, jobName string, _, _ time.Time, success bool, _ string) error {
		recorded = jobName + ":" + fmt.Sprint(success)
		return nil
	})

	if err := recorder.Record(context.Background(), "sync-widgets", time.Now(), time.Now(), true, ""); err != nil {
		panic(err)
	}

	fmt.Println(recorded)

	// Output:
	// sync-widgets:true
}
