package motospec

import (
	"context"
	"sync"
)

type Pipeline struct {
	ProcessorList []*Processor
	Ctx           context.Context
	Done          chan struct{}
	Input         chan interface{}
	Output        chan interface{}
	Error         chan error
	WG            sync.WaitGroup
}

func NewPipeline(ctx context.Context, pfList []ProcessFunc, interval int) *Pipeline {
	pipeLine := &Pipeline{
		ProcessorList: make([]*Processor, 0, 8),
		Ctx:           ctx,
		Done:          make(chan struct{}),
		Input:         make(chan interface{}),
		Error:         make(chan error),
	}
	firstProcessor := NewProcessor(pipeLine.Input, pipeLine.Error, ctx, pfList[0], interval)
	pipeLine.ProcessorList = append(pipeLine.ProcessorList, firstProcessor)
	for _, pf := range pfList[1:] {
		processor := NewProcessor(pipeLine.ProcessorList[len(pipeLine.ProcessorList)-1].Output, pipeLine.Error, ctx, pf, interval)
		pipeLine.ProcessorList = append(pipeLine.ProcessorList, processor)
	}
	pipeLine.Output = pipeLine.ProcessorList[len(pipeLine.ProcessorList)-1].Output
	return pipeLine
}

func (pl *Pipeline) Close() {
	for _, processor := range pl.ProcessorList {
		go func(p *Processor) {
			<-p.Done
			pl.WG.Done()
		}(processor)
	}
	pl.WG.Wait()
	close(pl.Error)
	close(pl.Done)
}

func (pl *Pipeline) Run() {
	for _, processor := range pl.ProcessorList {
		go processor.Run()
		pl.WG.Add(1)
	}
	go HandleError(pl.Error)
	pl.Close()
}
