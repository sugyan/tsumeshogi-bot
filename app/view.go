package app

import (
	"html/template"
	"net/http"
)

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) error {
	t, err := template.ParseFiles("templates/" + tmpl + ".html")
	if err != nil {
		return err
	}
	return t.Execute(w, data)
}
