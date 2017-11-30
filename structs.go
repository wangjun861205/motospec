package motospec

// type Manufacturer struct {
// 	Name  string
// 	Value string
// }
//
// type Category struct {
// 	Manufacturer Manufacturer
// 	Name         string
// 	Value        string
// }
//
// type Year struct {
// 	Category Category
// 	Name     string
// 	Value    string
// }
//
// type Model struct {
// 	Year  Year
// 	Name  string
// 	Value string
// }

// type Spec map[string]string

type BrandURL struct {
	Brand string
	URL   string
}

type ModelURL struct {
	Brand string
	Model string
	URL   string
}

type MotoURL struct {
	Brand string
	Model string
	Moto  string
	Year  string
	URL   string
}

type Spec struct {
	Brand string            `json:"brand"`
	Model string            `json:"model"`
	Moto  string            `json:"type"`
	Year  string            `json:"year"`
	Specs map[string]string `json:"specs"`
}
