package main

import "math/rand"

// ShuffleInPlace randomizes the order of the passed iterable in place
func shuffleInPlace[T any](input []T) {
	for i := len(input) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		input[i], input[j] = input[j], input[i]
	}
}
