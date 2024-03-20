package grtn

import "github.com/sirupsen/logrus"

var GlobalCount int

func Go(f func()) {
	go func() {
		logrus.Error("Number of goroutines: ", GlobalCount)
		GlobalCount++
		defer func() { GlobalCount-- }()
		f()
	}()
}

func GoA[T any](f func(T), arg T) {
	go func() {
		logrus.Error("Number of goroutines: ", GlobalCount)
		GlobalCount++
		defer func() { GlobalCount-- }()
		f(arg)
	}()
}

func GoA2[T1 any, T2 any](f func(T1, T2), arg1 T1, arg2 T2) {
	go func() {
		logrus.Error("Number of goroutines: ", GlobalCount)
		GlobalCount++
		defer func() { GlobalCount-- }()
		f(arg1, arg2)
	}()
}

func Goa3[T1 any, T2 any, T3 any](f func(T1, T2, T3), arg1 T1, arg2 T2, arg3 T3) {
	go func() {
		logrus.Error("Number of goroutines: ", GlobalCount)
		GlobalCount++
		defer func() { GlobalCount-- }()
		f(arg1, arg2, arg3)
	}()
}
