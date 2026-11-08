package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"taskwarrior/internal/tasks"
	"testing"
)

func benchmarkData(count int) tasks.Data {
	d := tasks.Empty()
	d.NextCompletionSeq = uint64(count/2 + 1)
	for i := 0; i < count; i++ {
		task := tasks.Task{ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1), Text: fmt.Sprintf("Task %05d", i), CreatedAt: "2026-11-07T20:00:00Z", UpdatedAt: "2026-11-07T20:00:00Z"}
		if i >= count/2 {
			task.Completed = true
			task.CompletionSeq = uint64(count - i)
		}
		d.Tasks = append(d.Tasks, task)
	}
	return d
}
func BenchmarkNavigation10000(b *testing.B) {
	for _, position := range []string{"top", "bottom"} {
		b.Run(position, func(b *testing.B) {
			m := New(benchmarkData(10000), "unused.json")
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			if position == "bottom" {
				m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := tea.KeyDown
				if (position == "top" && i%2 == 1) || (position == "bottom" && i%2 == 0) {
					key = tea.KeyUp
				}
				m.Update(tea.KeyPressMsg{Code: key})
				m.View()
			}
		})
	}
}
func BenchmarkStartup10000(b *testing.B) {
	data := benchmarkData(10000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := New(data, "unused.json")
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m.View()
	}
}
