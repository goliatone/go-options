package opts

import (
	"fmt"
	"testing"
)

func BenchmarkResolveWithTrace(b *testing.B) {
	layers := make([]Layer[traceSnapshot], 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("layer_%d", i)
		layers[i] = NewLayer(
			NewScope(name, 100-i),
			traceSnapshot{
				Name: name,
				Labels: map[string]string{
					"env": name,
				},
				Limits: map[string]int{
					"daily":  100 - i,
					"weekly": 700 - (i * 10),
				},
			},
		)
	}
	stack, err := NewStack(layers...)
	if err != nil {
		b.Fatalf("stack: %v", err)
	}
	opts, err := stack.Merge()
	if err != nil {
		b.Fatalf("merge: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := opts.ResolveWithTrace("Limits.weekly"); err != nil {
			b.Fatalf("resolve: %v", err)
		}
	}
}
