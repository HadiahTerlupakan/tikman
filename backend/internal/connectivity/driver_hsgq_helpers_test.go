package connectivity

import "fmt"

func ptr(v float64) *float64 { return &v }

func formatPtr(v *float64) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%.2f", *v)
}
