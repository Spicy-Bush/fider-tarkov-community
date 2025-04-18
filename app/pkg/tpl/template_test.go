package tpl_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/tpl"
)

func TestGetTemplate_Render(t *testing.T) {
	RegisterT(t)

	bf := new(bytes.Buffer)
	tmpl := tpl.GetTemplate("app/pkg/tpl/testdata/base.html", "app/pkg/tpl/testdata/echo.html")
	err := tpl.Render(context.Background(), tmpl, bf, dto.Props{
		"name": "John",
	})

	Expect(err).IsNil()
	Expect(bf.String()).ContainsSubstring(`Hello, John!`)
	Expect(bf.String()).ContainsSubstring(`This goes on the head.`)
}
