package cdktemplates

import "text/template"

func templateFiles() map[string]*template.Template {
	return map[string]*template.Template{
		"cdkapp.go": template.Must(template.New("cdkmain").Parse(CdkMainTmpl)),
		"go.mod":    template.Must(template.New("gomod").Parse(GoModTmpl)),
		"go.sum":    template.Must(template.New("gosum").Parse(GoSumTmpl)),
		"cdk.json":  template.Must(template.New("cdkjson").Parse(CdkJSONTmpl)),
	}
}
