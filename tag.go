package inject

import "github.com/facebookgo/structtag"

var (
	injectOnly    = &tag{}
	injectPrivate = &tag{Private: true}
	injectInline  = &tag{Inline: true}
)

type tag struct {
	Name    string
	Inline  bool
	Private bool
}

func parseTag(t string) (*tag, error) {
	found, value, err := structtag.Extract("inject", t)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	if value == "" {
		return injectOnly, nil
	}
	if value == "inline" {
		return injectInline, nil
	}
	if value == "private" {
		return injectPrivate, nil
	}
	return &tag{Name: value}, nil
}
