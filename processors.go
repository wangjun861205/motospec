package motospec

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"notbearclient"
	"notbearparser"
	"time"
)

type ProcessFunc func(*Processor, interface{})

type Processor struct {
	Client  *notbearclient.Client
	Input   chan interface{}
	Output  chan interface{}
	Error   chan error
	Done    chan struct{}
	Ctx     context.Context
	Process ProcessFunc

	Interval int
}

func NewProcessor(input chan interface{}, errChan chan error, ctx context.Context, processFunc ProcessFunc, interval int) *Processor {
	client := notbearclient.NewClient(3, 10, ctx, errChan)
	return &Processor{
		Client:  client,
		Input:   input,
		Output:  make(chan interface{}),
		Error:   errChan,
		Done:    make(chan struct{}),
		Ctx:     ctx,
		Process: processFunc,

		Interval: interval,
	}
}

func (p *Processor) Search(req *http.Request, query string) ([]*notbearparser.Node, error) {
	p.Client.Input <- req
	html := <-p.Client.Output
	parser := notbearparser.NewCursor(html)
	err := parser.Parse()
	if err != nil {
		return []*notbearparser.Node{}, err
	}
	nodes, err := notbearparser.Search(parser.Root, query)
	if err != nil {
		return []*notbearparser.Node{}, err
	}
	return nodes, nil
}

func (p *Processor) Close() {
	<-p.Client.Done
	fmt.Println("closing processor")
	close(p.Output)
	<-p.Input
	close(p.Done)
	fmt.Println("processor closed")
}

func (p *Processor) Run() {
	defer p.Close()
	go p.Client.Run()
	for {
		select {
		case <-p.Ctx.Done():
			return
		case input, ok := <-p.Input:
			if !ok {
				close(p.Client.Input)
				return
			}
			time.Sleep(time.Duration(p.Interval) * time.Second)
			p.Process(p, input)
		}
	}
}

var BrandFunc ProcessFunc = func(p *Processor, input interface{}) {
	s, ok := input.(string)
	if !ok {
		p.Error <- fmt.Errorf("%v is not valid url string", input)
		return
	}
	ProcessorLogger.Printf("Brand Processor: IN %s\n", s)
	req, err := notbearclient.NewRequest("GET", s, "", "motoSpecHeader", map[string][]string{})
	if err != nil {
		p.Error <- err
		return
	}
	nodes, err := p.Search(req, `.carman h5 a`)
	if err != nil {
		p.Error <- err
		return
	}
OUTER:
	for _, node := range nodes {
		select {
		case <-p.Ctx.Done():
			return
		default:
			brand := node.Children[0].Content
			hrefs, ok := node.Attrs.Get("href")
			if !ok {
				p.Error <- fmt.Errorf("%s has no link", brand)
				continue OUTER
			}
			p.Output <- BrandURL{Brand: brand, URL: hrefs[0]}
			ProcessorLogger.Printf("Brand Processor: OUT %s\n", s)
		}
	}
}

var ModelFunc ProcessFunc = func(p *Processor, input interface{}) {
	brand, ok := input.(BrandURL)
	if !ok {
		p.Error <- fmt.Errorf("%v is not valid BrandURL", input)
		return
	}
	ProcessorLogger.Printf("Model Processor: IN %s\n", brand.URL)
	req, err := notbearclient.NewRequest("GET", brand.URL, "", "motoSpecHeader", map[string][]string{})
	if err != nil {
		p.Error <- err
		return
	}
	nodes, err := p.Search(req, `.carmod a`)
	if err != nil {
		p.Error <- err
		return
	}
OUTER:
	for _, node := range nodes {
		select {
		case <-p.Ctx.Done():
			return
		default:
			models, err := notbearparser.Search(node, `h4`)
			if err != nil {
				p.Error <- err
				return
			}
			model := models[0].Content
			hrefs, ok := node.Attrs.Get("href")
			if !ok {
				p.Error <- fmt.Errorf("%s has no href", model)
				continue OUTER
			}
			p.Output <- ModelURL{Brand: brand.Brand, Model: model, URL: hrefs[0]}
			ProcessorLogger.Printf("Model Processor: OUT %s\n", brand.URL)
		}
	}
}

var MotoFunc ProcessFunc = func(p *Processor, input interface{}) {
	model, ok := input.(ModelURL)
	if !ok {
		p.Error <- fmt.Errorf("%v is not a valid ModelURL", input)
		return
	}
	ProcessorLogger.Printf("Moto Processor: IN %s\n", model.URL)
	req, err := notbearclient.NewRequest("GET", model.URL, "", "motoSpecHeader", map[string][]string{})
	if err != nil {
		p.Error <- err
		return
	}
	nodes, err := p.Search(req, `.carmodel`)
	if err != nil {
		p.Error <- err
		return
	}
OUTER:
	for _, node := range nodes {
		select {
		case <-p.Ctx.Done():
			return
		default:
			motoNames, err := notbearparser.Search(node, `span[itemprop="name"]`)
			if err != nil {
				p.Error <- err
				continue OUTER
			}
			moto := motoNames[0].Content
			years, err := notbearparser.Search(node, `p[class="years"]`)
			if err != nil {
				p.Error <- err
				continue OUTER
			}
			year := years[0].Content
			as, err := notbearparser.Search(node, `a[itemprop="url"]`)
			if err != nil {
				p.Error <- err
				continue OUTER
			}
			hrefs, ok := as[0].Attrs.Get("href")
			if !ok {
				p.Error <- fmt.Errorf("%s(%s) has no valid href", moto, year)
				continue OUTER
			}
			p.Output <- MotoURL{
				Brand: model.Brand,
				Model: model.Model,
				Moto:  moto,
				Year:  year,
				URL:   hrefs[0],
			}
			ProcessorLogger.Printf("Moto Processor: OUT %s", model.URL)
		}
	}
}

var SpecFunc ProcessFunc = func(p *Processor, input interface{}) {
	moto, ok := input.(MotoURL)
	if !ok {
		p.Error <- fmt.Errorf("%v is not a valid MotoURL", input)
	}
	ProcessorLogger.Printf("Spec Processor: IN %s", moto.URL)
	req, err := notbearclient.NewRequest("GET", moto.URL, "", "motoSpecHeader", map[string][]string{})
	if err != nil {
		p.Error <- err
		return
	}
	specTabs, err := p.Search(req, `.enginedata`)
	if err != nil {
		p.Error <- err
		return
	}
	if len(specTabs) == 0 {
		p.Error <- fmt.Errorf("%s %s %s(%s) has no spec table", moto.Brand, moto.Model, moto.Moto, moto.Year)
		return
	}
	dts, err := notbearparser.Search(specTabs[0], `dt em`)
	if err != nil {
		p.Error <- err
		return
	}
	dds, err := notbearparser.Search(specTabs[0], `dd`)
	if err != nil {
		p.Error <- err
		return
	}
	if len(dts) != len(dds) {
		p.Error <- errors.New("spec table dt is not equal to dd")
		return
	}
	spec := Spec{
		Brand: moto.Brand,
		Model: moto.Model,
		Moto:  moto.Moto,
		Year:  moto.Year,
		Specs: make(map[string]string),
	}
	for i := 0; i < len(dts); i++ {
		spec.Specs[dts[i].Content] = dds[i].Content
	}
	p.Output <- spec
	ProcessorLogger.Printf("Spec Processor: OUT %s", moto.URL)
}

// type ManufacturerProcessor struct {
// 	Client       *notbearclient.Client
// 	ClientCancel context.CancelFunc
// 	Output       chan Manufacturer
// 	Error        chan error
// 	Done         chan struct{}
// 	Ctx          context.Context
// }
//
// func NewManufacturerProcessor(err chan error, ctx context.Context) *ManufacturerProcessor {
// 	newCtx, cancel := context.WithCancel(context.Background())
// 	client := notbearclient.NewClient(3, 10, newCtx, err)
// 	return &ManufacturerProcessor{
// 		Client:       client,
// 		ClientCancel: cancel,
// 		Output:       make(chan Manufacturer, 128),
// 		Error:        err,
// 		Done:         make(chan struct{}, 1),
// 		Ctx:          ctx,
// 	}
// }
//
// func (mfp *ManufacturerProcessor) Run() {
// 	defer mfp.Close()
// 	go mfp.Client.Run()
// OUTER:
// 	for {
// 		select {
// 		case <-mfp.Ctx.Done():
// 			return
// 		default:
// 			url := fmt.Sprintf(BaseURL, "0", "", "", "")
// 			req, err := notbearclient.NewRequest("GET", url, "", "motoSpecHeader", map[string][]string{})
// 			if err != nil {
// 				mfp.Error <- fmt.Errorf("failed to get %s", url)
// 				return
// 			}
// 			mfp.Client.Input <- req
// 			if content, ok := <-mfp.Client.Output; ok {
// 				cursor := notbearparser.NewCursor(content)
// 				err = cursor.Parse()
// 				if err != nil {
// 					mfp.Error <- err
// 					return
// 				}
// 				options, err := notbearparser.Search(cursor.Root, "option")
// 				if err != nil {
// 					mfp.Error <- err
// 					return
// 				}
// 				if len(options) == 0 {
// 					mfp.Error <- fmt.Errorf("Empty manufacturer option(%s)", url)
// 					return
// 				}
// 				for _, option := range options {
// 					if value, ok := option.Attrs.Get("value"); ok && value[0] != "" {
// 						name := strings.Trim(option.Content, "\n\r ")
// 						manufacturer := Manufacturer{Name: name, Value: value[0]}
// 						mfp.Output <- manufacturer
// 					}
// 				}
// 				break OUTER
// 			}
// 		}
// 	}
// }
//
// func (mfp *ManufacturerProcessor) Close() {
// 	fmt.Println("Closing manufacturer processor......")
// 	mfp.ClientCancel()
// 	<-mfp.Client.Done
// 	close(mfp.Output)
// 	mfp.Done <- struct{}{}
// 	close(mfp.Done)
// 	fmt.Println("Manufacturer processor has closed")
// }
//
// func (mfp *ManufacturerProcessor) DoneChan() chan struct{} {
// 	return mfp.Done
// }
//
// type CategoryProcessor struct {
// 	Client       notbearclient.Client
// 	ClientCancel context.CancelFunc
// 	Input        chan Manufacturer
// 	Output       chan Category
// 	Error        chan error
// 	Done         chan struct{}
// 	Ctx          context.Context
// }
//
// func NewCategoryProcessor(input chan Manufacturer, errChan chan error, ctx context.Context) *CategoryProcessor {
// 	newCtx, cancel := context.WithCancel(context.Background())
// 	client := notbearclient.NewClient(3, 10, newCtx, errChan)
// 	return &CategoryProcessor{
// 		Client:       *client,
// 		ClientCancel: cancel,
// 		Input:        input,
// 		Output:       make(chan Category),
// 		Error:        errChan,
// 		Done:         make(chan struct{}),
// 		Ctx:          ctx,
// 	}
// }
//
// func (ctp *CategoryProcessor) Close() {
// 	fmt.Println("Closing category processor......")
// 	ctp.ClientCancel()
// 	<-ctp.Client.Done
// 	close(ctp.Output)
// 	ctp.Done <- struct{}{}
// 	close(ctp.Done)
// 	fmt.Println("Category processor has closed")
// }
//
// func (ctp *CategoryProcessor) DoneChan() chan struct{} {
// 	return ctp.Done
// }
//
// func (ctp *CategoryProcessor) Run() {
// 	defer ctp.Close()
// 	go ctp.Client.Run()
// OUTER:
// 	for {
// 		select {
// 		case <-ctp.Ctx.Done():
// 			return
// 		case mf, ok := <-ctp.Input:
// 			if ok {
// 				url := fmt.Sprintf(BaseURL, "1", mf.Value, "", "")
// 				req, err := notbearclient.NewRequest("GET", url, "", "motoSpecHeader", map[string][]string{})
// 				if err != nil {
// 					ctp.Error <- err
// 					continue OUTER
// 				}
// 				ctp.Client.Input <- req
// 				result, ok := <-ctp.Client.Output
// 				if ok {
// 					parser := notbearparser.NewCursor(result)
// 					err = parser.Parse()
// 					if err != nil {
// 						ctp.Error <- err
// 						continue OUTER
// 					}
// 					l, err := notbearparser.Search(parser.Root, `option`)
// 					if err != nil {
// 						ctp.Error <- err
// 						continue OUTER
// 					}
// 					for _, node := range l {
// 						if value, ok := node.Attrs.Get("value"); ok && value[0] != "" {
// 							category := Category{Manufacturer: mf, Name: node.Content, Value: value[0]}
// 							ctp.Output <- category
// 						}
// 					}
// 				}
// 			} else {
// 				break OUTER
// 			}
// 		}
// 	}
// }
//
// type YearProcessor struct {
// 	Client       *notbearclient.Client
// 	ClientCancel context.CancelFunc
// 	Input        <-chan Category
// 	Output       chan Year
// 	Error        chan<- error
// 	Done         chan struct{}
// 	Ctx          context.Context
// }
//
// func NewYearProcessor(input <-chan Category, errChan chan error, ctx context.Context) *YearProcessor {
// 	clientCtx, cancel := context.WithCancel(context.Background())
// 	client := notbearclient.NewClient(3, 10, clientCtx, errChan)
// 	return &YearProcessor{
// 		Client:       client,
// 		ClientCancel: cancel,
// 		Input:        input,
// 		Output:       make(chan Year),
// 		Error:        errChan,
// 		Done:         make(chan struct{}),
// 		Ctx:          ctx,
// 	}
// }
//
// func (yp *YearProcessor) Close() {
// 	fmt.Println("Closing year processor......")
// 	yp.ClientCancel()
// 	<-yp.Client.Done
// 	close(yp.Output)
// 	yp.Done <- struct{}{}
// 	close(yp.Done)
// 	fmt.Println("Year processor has closed")
// }
//
// func (yp *YearProcessor) DoneChan() chan struct{} {
// 	return yp.Done
// }
//
// func (yp *YearProcessor) Run() {
// 	defer yp.Close()
// 	go yp.Client.Run()
// OUTER:
// 	for {
// 		select {
// 		case <-yp.Ctx.Done():
// 			return
// 		case category, ok := <-yp.Input:
// 			if ok {
// 				url := fmt.Sprintf(BaseURL, "2", category.Manufacturer.Value, category.Value, "")
// 				req, err := notbearclient.NewRequest("GET", url, "", "motoSpecHeader", map[string][]string{})
// 				if err != nil {
// 					yp.Error <- err
// 					continue OUTER
// 				}
// 				yp.Client.Input <- req
// 				if result, ok := <-yp.Client.Output; ok {
// 					parser := notbearparser.NewCursor(result)
// 					err = parser.Parse()
// 					if err != nil {
// 						yp.Error <- err
// 						continue OUTER
// 					}
// 					l, err := notbearparser.Search(parser.Root, "option")
// 					if err != nil {
// 						yp.Error <- err
// 						continue OUTER
// 					}
// 					for _, node := range l {
// 						if values, ok := node.Attrs.Get("value"); ok && values[0] != "" {
// 							year := Year{Category: category, Name: node.Content, Value: values[0]}
// 							yp.Output <- year
// 						}
// 					}
// 				}
// 			} else {
// 				return
// 			}
// 		}
// 	}
// }
//
// type ModelProcessor struct {
// 	Client       notbearclient.Client
// 	ClientCancel context.CancelFunc
// 	Input        <-chan Year
// 	Output       chan Model
// 	Error        chan error
// 	Done         chan struct{}
// 	Ctx          context.Context
// }
//
// func NewModelProcessor(input <-chan Year, errChan chan error, ctx context.Context) *ModelProcessor {
// 	clientCtx, cancel := context.WithCancel(context.Background())
// 	client := notbearclient.NewClient(3, 10, clientCtx, errChan)
// 	return &ModelProcessor{
// 		Client:       *client,
// 		ClientCancel: cancel,
// 		Input:        input,
// 		Output:       make(chan Model),
// 		Error:        errChan,
// 		Done:         make(chan struct{}),
// 		Ctx:          ctx,
// 	}
// }
//
// func (mp *ModelProcessor) Close() {
// 	fmt.Println("Closing model processor......")
// 	mp.ClientCancel()
// 	<-mp.Client.Done
// 	close(mp.Output)
// 	mp.Done <- struct{}{}
// 	close(mp.Done)
// 	fmt.Println("Model processor has closed")
// }
//
// func (mp *ModelProcessor) DoneChan() chan struct{} {
// 	return mp.Done
// }
//
// func (mp *ModelProcessor) Run() {
// 	defer mp.Close()
// 	go mp.Client.Run()
// OUTER:
// 	for {
// 		select {
// 		case <-mp.Ctx.Done():
// 			return
// 		case year, ok := <-mp.Input:
// 			if ok {
// 				url := fmt.Sprintf(BaseURL, "3", year.Category.Manufacturer.Value, year.Category.Value, year.Value)
// 				req, err := notbearclient.NewRequest("GET", url, "", "motoSpecHeader", map[string][]string{})
// 				if err != nil {
// 					mp.Error <- err
// 					continue OUTER
// 				}
// 				mp.Client.Input <- req
// 				if result, ok := <-mp.Client.Output; ok {
// 					parser := notbearparser.NewCursor(result)
// 					err = parser.Parse()
// 					if err != nil {
// 						mp.Error <- err
// 						continue OUTER
// 					}
// 					l, err := notbearparser.Search(parser.Root, "option")
// 					if err != nil {
// 						mp.Error <- err
// 						continue OUTER
// 					}
// 					for _, node := range l {
// 						if values, ok := node.Attrs.Get("value"); ok && values[0] != "" {
// 							model := Model{Year: year, Name: node.Content, Value: values[0]}
// 							mp.Output <- model
// 						}
// 					}
// 				}
// 			} else {
// 				return
// 			}
// 		}
// 	}
// }
//
// type SpecProcessor struct {
// 	Client       notbearclient.Client
// 	ClientCancel context.CancelFunc
// 	Input        <-chan Model
// 	Output       chan Spec
// 	Error        chan error
// 	Done         chan struct{}
// 	Ctx          context.Context
// }
//
// func NewSpecProcessor(input <-chan Model, errChan chan error, ctx context.Context) *SpecProcessor {
// 	clientCtx, cancel := context.WithCancel(context.Background())
// 	client := notbearclient.NewClient(3, 10, clientCtx, errChan)
// 	return &SpecProcessor{
// 		Client:       *client,
// 		ClientCancel: cancel,
// 		Input:        input,
// 		Output:       make(chan Spec),
// 		Error:        errChan,
// 		Done:         make(chan struct{}),
// 		Ctx:          ctx,
// 	}
// }
//
// func (sp *SpecProcessor) Close() {
// 	fmt.Println("Closing spec processor......")
// 	sp.ClientCancel()
// 	<-sp.Client.Done
// 	close(sp.Output)
// 	sp.Done <- struct{}{}
// 	close(sp.Done)
// 	fmt.Println("Spec processor has closed")
// }
//
// func (sp *SpecProcessor) DoneChan() chan struct{} {
// 	return sp.Done
// }
//
// func (sp *SpecProcessor) Run() {
// 	defer sp.Close()
// 	go sp.Client.Run()
// OUTER:
// 	for {
// 		select {
// 		case <-sp.Ctx.Done():
// 			return
// 		case model, ok := <-sp.Input:
// 			if ok {
// 				req, err := notbearclient.NewRequest("POST", "http://www.motorcycle.com/specs/", "application/x-www-form-urlencoded", "motoSpecHeader", map[string][]string{
// 					"MakeId":      []string{model.Year.Category.Manufacturer.Value},
// 					"ModelType":   []string{model.Year.Category.Value},
// 					"year":        []string{model.Year.Value},
// 					"TrimId":      []string{model.Value},
// 					"get_specs.x": []string{"94"},
// 					"get_specs.y": []string{"14"},
// 				})
// 				if err != nil {
// 					sp.Error <- err
// 					continue OUTER
// 				}
// 				sp.Client.Input <- req
// 				if result, ok := <-sp.Client.Output; ok {
// 					parser := notbearparser.NewCursor(result)
// 					err = parser.Parse()
// 					if err != nil {
// 						sp.Error <- err
// 						continue OUTER
// 					}
// 					l, err := notbearparser.Search(parser.Root, ".table_info td")
// 					if err != nil {
// 						sp.Error <- err
// 						continue OUTER
// 					}
// 					spec := Spec{}
// 					spec["Manufacturer"] = model.Year.Category.Manufacturer.Name
// 					spec["Category"] = model.Year.Category.Name
// 					spec["Year"] = model.Year.Name
// 					spec["Model"] = model.Name
// 					for i := 0; i < len(l)-1; i += 2 {
// 						key := l[i].Content
// 						value := l[i+1].Content
// 						spec[key] = value
// 					}
// 					sp.Output <- spec
// 				}
// 			} else {
// 				return
// 			}
// 		}
// 	}
// }
