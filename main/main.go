package main

import (
	"context"
	"encoding/json"
	"fmt"
	"motospec"
	"os"
	"os/signal"
)

var StartURL = "https://www.autoevolution.com/moto/"

func HandleInterrupt(pl *motospec.Pipeline, cancel context.CancelFunc) {
	interruptChan := make(chan os.Signal)
	signal.Notify(interruptChan, os.Interrupt)
	<-interruptChan
	fmt.Println("Got a interrupt signal, closing......")
	cancel()
}

func main() {
	jsonFile, err := os.OpenFile("motospecs.json", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		fmt.Println(err)
		return
	}
	jsonEncoder := json.NewEncoder(jsonFile)
	ctx, cancel := context.WithCancel(context.Background())
	pipeline := motospec.NewPipeline(ctx, []motospec.ProcessFunc{
		motospec.BrandFunc,
		motospec.ModelFunc,
		motospec.MotoFunc,
		motospec.SpecFunc,
	}, 4)
	go pipeline.Run()
	pipeline.Input <- StartURL
	close(pipeline.Input)
	go HandleInterrupt(pipeline, cancel)
	for s := range pipeline.Output {
		spec := s.(motospec.Spec)
		jsonEncoder.Encode(spec)
		fmt.Println(spec.Brand, spec.Model, spec.Moto, spec.Year)
	}
	<-pipeline.Done
}
