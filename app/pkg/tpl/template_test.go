package tpl_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/crypto"
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

func TestCustomStylesCannotCreateHTML(t *testing.T) {
	tmpl := tpl.GetTemplate("views/base.html", "views/index.html")
	for _, css := range []string{
		`body { color: rgb(10, 20, 30); }`,
		`body::before { content: "a  b /* literal */"; }`,
		`body::before { content: "</style><img id=css-markup>"; }`,
		`</StYlE ><form id=css-markup action=/api/posts>`,
	} {
		t.Run(css, func(t *testing.T) {
			var output bytes.Buffer
			err := tpl.Render(context.Background(), tmpl, &output, dto.Props{
				"public": dto.Props{
					"tenant": &entity.Tenant{ID: 1, CustomCSS: css},
					"settings": dto.Props{
						"locale": "en", "googleAdSense": "", "googleAnalytics": "",
					},
				},
				"private": dto.Props{},
			})
			if err != nil {
				t.Fatal(err)
			}

			if strings.Contains(output.String(), "css-markup") {
				t.Fatal("custom CSS was embedded as document markup")
			}
			stylesheet := `/static/custom/` + crypto.MD5(css) + `.css`
			if !strings.Contains(output.String(), `href="`+stylesheet+`"`) {
				t.Fatal("custom stylesheet is missing its content-addressed URL")
			}
		})
	}
}
