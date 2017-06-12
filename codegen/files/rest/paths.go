package rest

import (
	"fmt"
	"path/filepath"
	"text/template"

	"goa.design/goa.v2/codegen"
	"goa.design/goa.v2/codegen/files"
	"goa.design/goa.v2/design"
	"goa.design/goa.v2/design/rest"
)

type (
	// PathData contains the data necessary to render the path template.
	PathData struct {
		// ServiceName is the name of the service defined in the design.
		ServiceName string
		// MethodName is the name of the method defined in the design.
		MethodName string
		// Routes describes all the possible paths for an action.
		Routes []*PathRoute
	}

	// PathRoute contains the data to render a path for a specific route.
	PathRoute struct {
		// Path is the fullpath converted to printf compatible layout.
		Path string
		// PathParams are all the path parameters in this route.
		PathParams []string
		// Arguments describe the arguments used in the route.
		Arguments []*PathArgument
	}

	// PathArgument contains the name and data type of the path arguments.
	PathArgument struct {
		// Name is the name of the argument variable.
		Name string
		// Type describes the datatype of the argument.
		Type design.DataType
	}

	// pathFile is the codegen file that generates the path constructors.
	pathFile struct {
		sections []*codegen.Section
	}
)

// PathFile returns the path file.
func PathFile(r *rest.RootExpr) codegen.File {
	api := r.Design.API
	path := filepath.Join("transport", "http", "paths.go")
	title := fmt.Sprintf("%s HTTP request path constructors", api.Name)
	sections := func(_ string) []*codegen.Section {
		s := []*codegen.Section{
			codegen.Header(title, "http", []*codegen.ImportSpec{
				{Path: "fmt"},
				{Path: "net/url"},
				{Path: "strconv"},
				{Path: "strings"},
			}),
		}

		for _, res := range r.Resources {
			for _, a := range res.Actions {
				s = append(s, PathSection(a))
			}
		}
		return s
	}

	return codegen.NewSource(path, sections)
}

// PathSection returns the section to generate the given paht.
func PathSection(a *rest.ActionExpr) *codegen.Section {
	return &codegen.Section{
		Template: pathTmpl(a.Resource),
		Data:     buildPathData(a),
	}
}

// pathTmpl returns the template used to render the paths functions.
func pathTmpl(r *rest.ResourceExpr) *template.Template {
	svc := files.Services.Get(r.Name())
	return template.Must(template.New("path").
		Funcs(template.FuncMap{
			"add":       codegen.Add,
			"goTypeRef": svc.Scope.GoTypeRef,
			"goify":     codegen.Goify,
			"isArray":   design.IsArray,
		}).
		Parse(pathT))
}

func buildPathData(a *rest.ActionExpr) *PathData {
	pd := PathData{
		ServiceName: a.MethodExpr.Service.Name,
		MethodName:  a.Name(),
		Routes:      make([]*PathRoute, len(a.Routes)),
	}

	for i, r := range a.Routes {
		pd.Routes[i] = &PathRoute{
			Path:       rest.WildcardRegex.ReplaceAllString(r.FullPath(), "/%v"),
			PathParams: r.ParamAttributes(),
			Arguments:  generatePathArguments(r),
		}
	}
	return &pd
}

func generatePathArguments(r *rest.RouteExpr) []*PathArgument {
	routeParams := r.ParamAttributes()
	allParams := r.Action.PathParams()
	args := make([]*PathArgument, len(routeParams))
	for i, name := range routeParams {
		args[i] = &PathArgument{
			Name: name,
			Type: allParams.Type.(design.Object)[name].Type,
		}
	}
	return args
}

const pathT = `{{ range $i, $route := .Routes -}}
// {{ goify $.MethodName true }}{{ goify $.ServiceName true }}Path{{ if ne $i 0 }}{{ add $i 1 }}{{ end }} returns the URL path to the {{ $.ServiceName }} service {{ $.MethodName }} HTTP endpoint.
func {{ goify $.MethodName true }}{{ goify $.ServiceName true }}Path{{ if ne $i 0 }}{{ add $i 1 }}{{ end }}({{ template "arguments" .Arguments }}) string {
{{- if .Arguments }}
	{{ template "slice_conversion" .Arguments -}}
	return fmt.Sprintf("{{ .Path }}"{{ template "fmt_params" .Arguments }})
{{- else }}
	return "{{ .Path }}"
{{- end }}
}

{{ end }}

{{- define "arguments" -}}
{{ range $i, $arg := . -}}
{{ if ne $i 0 }}, {{ end }}{{ goify .Name false }} {{ goTypeRef .Type }}
{{- end }}
{{- end }}

{{- define "fmt_params" -}}
{{ range . -}}
, {{ if isArray .Type }}strings.Join(encoded{{ goify .Name true }}, ",")
  {{- else }}{{ goify .Name false }}{{ end }}
{{- end }}
{{- end }}

{{- define "slice_conversion" -}}
{{ range $i, $arg := . }}
	{{- if isArray .Type -}}
	encoded{{ goify .Name true }} := make([]string, len({{ goify .Name false }}))
	{{- $elemType := .Type.ElemType.Type.Name }}
	for i, v := range {{ goify .Name false }} {
		encoded{{ goify .Name true }}[i] = {{ if eq $elemType "string" }}url.QueryEscape(v)
	{{ else if eq $elemType "int" "int32"   }}strconv.FormatInt(int64(v), 10)
	{{ else if eq $elemType "int64"         }}strconv.FormatInt(v, 10)
	{{ else if eq $elemType "uint" "uint32" }}strconv.FormatUint(uint64(v), 10)
	{{ else if eq $elemType "uint64"        }}strconv.FormatUint(v, 10)
	{{ else if eq $elemType "float32"       }}strconv.FormatFloat(float64(v), 'f', -1, 32)
	{{ else if eq $elemType "float64"       }}strconv.FormatFloat(v, 'f', -1, 64)
	{{ else if eq $elemType "boolean"       }}strconv.FormatBool(v)
	{{ else if eq $elemType "bytes"         }}url.QueryEscape(string(v))
	{{ else }}url.QueryEscape(fmt.Sprintf("%v", v))
	{{ end -}}
	}

	{{ end }}
{{- end }}
{{- end }}`
