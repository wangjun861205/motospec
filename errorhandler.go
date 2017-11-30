package motospec

import (
	"notbearclient"
)

func HandleError(errChan chan error) {
	for err := range errChan {
		switch e := err.(type) {
		case *notbearclient.ErrTimeout:
			ErrTimeoutLogger.Println(e)
		case *notbearclient.ErrFailed:
			ErrFailedLogger.Println(e)
		default:
			ErrProcessorLogger.Println(e)
		}
	}
}
