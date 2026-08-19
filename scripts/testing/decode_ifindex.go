package main
import "fmt"

func main() {
	ifindex := 268632320
	
	rack := (ifindex >> 25) & 0x7F
	shelf := (ifindex >> 19) & 0x3F
	slot := (ifindex >> 13) & 0x3F
	port := (ifindex >> 8) & 0x1F
	
	fmt.Printf("ifindex: %d (0x%X)\n", ifindex, ifindex)
	fmt.Printf("Decoded:\n")
	fmt.Printf("  Rack:  %d\n", rack)
	fmt.Printf("  Shelf: %d\n", shelf)
	fmt.Printf("  Slot:  %d\n", slot)
	fmt.Printf("  Port:  %d\n", port)
}
