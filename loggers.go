package motospec

import (
	"log"
	"os"
)

// var ErrTimeoutLogger *log.Logger
// var ErrFailedLogger *log.Logger
var ErrClientLogger *log.Logger
var ErrProcessorLogger *log.Logger
var ProcessorLogger *log.Logger

func init() {
	// errTimeoutFile, err := os.OpenFile("errTimeout.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	// if err != nil {
	// 	panic(err)
	// }
	// errFailedFile, err := os.OpenFile("errFailed.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	// if err != nil {
	// 	panic(err)
	// }
	errClientFile, err := os.OpenFile("errClient.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0664)
	if err != nil {
		panic(err)
	}
	errProcessorFile, err := os.OpenFile("errProcessor.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		panic(err)
	}
	processorFile, err := os.OpenFile("processor.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)

	// ErrTimeoutLogger = log.New(errTimeoutFile, "Client timeout:", log.Ldate|log.Ltime)
	// ErrFailedLogger = log.New(errFailedFile, "Client failed:", log.Ldate|log.Ltime)
	ErrClientLogger = log.New(errClientFile, "Client error:", log.Ldate|log.Ltime)
	ErrProcessorLogger = log.New(errProcessorFile, "Processor error:", log.Ldate|log.Ltime)
	ProcessorLogger = log.New(processorFile, "Processor:", log.Ldate|log.Ltime)
}
