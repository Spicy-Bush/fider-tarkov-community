package tpl

import (
	"context"
	"html/template"
	"io"
	"path"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
)

func GetTemplate(baseFileName, templateFileName string) *template.Template {
	baseFile := strings.TrimPrefix(baseFileName, "/")
	templateFile := strings.TrimPrefix(templateFileName, "/")

	tpl, err := template.New(path.Base(baseFile)).Funcs(templateFunctions).ParseFS(assets.FS, baseFile, templateFile)
	if err != nil {
		panic(errors.Wrap(err, "failed to parse template %s", templateFileName))
	}

	return tpl
}

func Render(ctx context.Context, tmpl *template.Template, w io.Writer, data any) error {
	if err := template.Must(tmpl.Clone()).Funcs(template.FuncMap{
		"translate": func(key string, params ...i18n.Params) string {
			return i18n.T(ctx, key, params...)
		},
	}).Execute(w, data); err != nil {
		return err
	}
	return nil
}
