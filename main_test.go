package main

import (
	"reflect"
	"testing"
)

func TestTwoSum(t *testing.T) {
	tests := []struct {
		name   string
		nums   []int
		target int
		want   []int
	}{
		{"example 1", []int{2, 7, 11, 15}, 9, []int{0, 1}},
		{"example 2", []int{3, 2, 4}, 6, []int{1, 2}},
		{"duplicates", []int{3, 3}, 6, []int{0, 1}},
		{"negative numbers", []int{-1, -2, -3, -4, -5}, -8, []int{2, 4}},
		{"mixed signs", []int{-3, 4, 3, 90}, 0, []int{0, 2}},
		{"zeros", []int{0, 4, 3, 0}, 0, []int{0, 3}},
		{"constraint boundaries", []int{-1000000000, 1000000000}, 0, []int{0, 1}},
		{"cannot reuse element", []int{3, 2, 5}, 6, nil},
		{"empty", nil, 0, nil},
		{"one element", []int{3}, 6, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := twoSum(tt.nums, tt.target); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("twoSum(%v, %d) = %v, want %v", tt.nums, tt.target, got, tt.want)
			}
		})
	}
}
