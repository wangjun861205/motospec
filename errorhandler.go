package motospec

import (
	"notbearclient"
)

func HandleError(errChan chan error) {
	for err := range errChan {
		switch e := err.(type) {
		case *notbearclient.ErrTimeout, *notbearclient.ErrNetwork, *notbearclient.ErrOther:
			ErrClientLogger.Println(e)
		default:
			ErrProcessorLogger.Println(e)
		}
	}
}
