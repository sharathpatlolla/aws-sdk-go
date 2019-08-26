// +build codegen

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"
)

type BehaviorTestSuite struct {
	Defaults Defaults `json:"defaults"`
	Tests Tests `json:"tests"`
}

type Tests struct {
	Defaults Defaults `json:"defaults"`
	Cases []Case `json:"cases"`
}

type Defaults struct{
	Env map[string]string `json:"env"`
	Files interface{} `json:"files"`
	Config interface{} `json:"config"`
}

type Case struct{
	Description string `json:"description"`
	LocalConfig map[string]string `json:"localConfig"`
	Request Request `json:"request"`
	Response Response `json:"response"`
	Expect []map[string]interface{} `json:"expect"`
}

type Response struct{
	StatusCode int `json:"statusCode"`
	BodyContent string `json:"bodyContent"`
	BodyType string `json:"bodyType"`
	Headers map[string]string `json:"headers"`

}

type Request struct{
	Operation string `json:"operation"`
	Input map[string]interface{} `json:"input"`
}

func (c Request) BuildInputShape(ref *ShapeRef) string {
	b := ShapeValueBuilder{IsBase64:true}
	return fmt.Sprintf("&%s{\n%s\n}",
		b.GoType(ref, true),
		b.BuildShape(ref, c.Input, false),
	)
}

//Outputs the string to define an empty shape
func (c Request) EmptyShapeBuilder(ref *ShapeRef) string{
	var b ShapeValueBuilder
	return fmt.Sprintf("%s{}", b.GoType(ref, true))
}


func (c Case) BuildOutputShape(ref *ShapeRef) string{
	b := ShapeValueBuilder{IsBase64:true}
	return fmt.Sprintf("&%s{\n%s\n}",
		b.GoType(ref, true),
		b.BuildShape(ref, c.Expect[0]["responseDataEquals"].(map[string]interface{}), false),
	)
}

// AttachBehaviorTests attaches the Behavior test cases to the API model.
func (a *API) AttachBehaviorTests(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		panic(fmt.Sprintf("failed to open behavior tests %s, err: %v", filename, err))
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&a.BehaviorTests); err != nil {
		panic(fmt.Sprintf("failed to decode behavior tests %s, err: %v", filename, err))
	}

}

// APIBehaviorTestsGoCode returns the Go Code string for the Behavior tests.
func (a *API) APIBehaviorTestsGoCode() string {
	w := bytes.NewBuffer(nil)
	a.resetImports()

	a.AddImport("testing")
	a.AddImport("net/http")
	a.AddImport("time")
	a.AddImport("io/ioutil")
	a.AddImport("bytes")
	a.AddImport("strings")

	a.AddSDKImport("aws")
	a.AddSDKImport("awstesting")
	a.AddSDKImport("aws/session")
	a.AddSDKImport("aws/credentials")
	a.AddSDKImport("aws/corehandlers")
	a.AddSDKImport("aws/request")
	a.AddSDKImport("private/protocol")
	a.AddSDKImport("internal/sdktesting")

	a.AddImport(a.ImportPath())

	behaviorTests := struct {
		API *API
		BehaviorTestSuite
	}{
		API:            a,
		BehaviorTestSuite: a.BehaviorTests,
	}

	if err := behaviorTestTmpl.Execute(w, behaviorTests); err != nil {
		panic(fmt.Sprintf("failed to create behavior tests, %v", err))
	}

	return a.importsGoCode()  + w.String()
}

// Changes the first character of val to upper case
func FormatAssertionName (val string) string{
	tempVal := []byte(val)
	tempVal[0] -= 32 //First Letter to UpperCase
	return string(tempVal)
}

//Generates assertions
func (c Case) GenerateAssertions (op *Operation) string{
	var val string = "//Assertions start here"

	for _, assertion := range  c.Expect{
		for assertionName, assertionContext := range assertion{
			if assertionName  == "responseErrorDataEquals" {
				val += fmt.Sprintf("\nawstesting.Assert%s(t, %#v, err)", FormatAssertionName(assertionName), assertionContext)
				continue
			}
			val += fmt.Sprintf("\n")

			val += "if !awstesting.Assert"
			switch assertionName {
			case "requestMethodEquals":
				val += fmt.Sprintf("%s(t, %q, req.HTTPRequest.Method)",FormatAssertionName(assertionName),assertionContext)
			case "requestUrlMatches":
				val += fmt.Sprintf("%s(t, %q, req.HTTPRequest.URL.String())",FormatAssertionName(assertionName),assertionContext)
			case "requestUrlPathMatches":
				val += fmt.Sprintf("%s(t, %q, req.HTTPRequest.URL.EscapedPath())",FormatAssertionName(assertionName),assertionContext)
			case "requestUrlQueryMatches":
				val += fmt.Sprintf("%s(t, %q, req)",FormatAssertionName(assertionName),assertionContext)
			case "requestHeadersMatch":
				val += fmt.Sprintf("%s(t, %#v, req)",FormatAssertionName(assertionName),assertionContext)
			case "requestBodyEqualsBytes":
				val += fmt.Sprintf("%s(t, %q, req)",FormatAssertionName(assertionName),assertionContext)
			case "requestBodyEqualsJson":
				val += fmt.Sprintf("%s(t, %#v, req)",FormatAssertionName(assertionName),assertionContext)
			case "requestBodyMatchesXml":
				val += fmt.Sprintf("%s(t, %q, req, %v)",FormatAssertionName(assertionName), assertionContext,c.Request.EmptyShapeBuilder(&op.InputRef))
			case "requestBodyEqualsString":
				val += fmt.Sprintf("%s(t, %q, req)",FormatAssertionName(assertionName),assertionContext)
			case "requestIdEquals":
				val += fmt.Sprintf("%s(t, %q, req.RequestID)",FormatAssertionName(assertionName),assertionContext)
			case "responseDataEquals":
				val += fmt.Sprintf("%s(t, %v, resp)",FormatAssertionName(assertionName),c.BuildOutputShape(&op.OutputRef))
			case "responseErrorIsKindOf":
				val += fmt.Sprintf("%s(t, %q, err)",FormatAssertionName(assertionName),assertionContext)
			case "responseErrorMessageEquals":
				val += fmt.Sprintf("%s(t, %q, err)",FormatAssertionName(assertionName),assertionContext)
			case "responseErrorRequestIdEquals":
				val += fmt.Sprintf("%s(t, %q, err)",FormatAssertionName(assertionName),assertionContext)
			default:
				val += fmt.Sprintf("%s(t, %q, err)",FormatAssertionName(assertionName),assertionContext)
			}

			val += fmt.Sprintf(`{ 
				t.Errorf("Expect no error, got %s assertion failed")
			}`,assertionName)

		}
	}
	return val
}

//template map is defined in "eventstream.go"
var funcMap = template.FuncMap{"Map": templateMap,"FormatAssertionName": FormatAssertionName}

var behaviorTestTmpl = template.Must(template.New(`behaviorTestTmpl`).Funcs(funcMap).Parse(`

{{define "StashCredentials"}}
	restoreEnv := sdktesting.StashEnv() //Stashes the current environment
	defer restoreEnv()
{{end}}

{{define "SessionSetup"}}
	//Starts a new session with credentials and region parsed from "defaults" in the Json file'
	sess := session.Must(session.NewSession(&aws.Config{
			 Region: aws.String( {{- if and (len $.testCase.LocalConfig) $.testCase.LocalConfig.AWS_REGION }} "{{$.testCase.LocalConfig.AWS_REGION}}" {{- else}} "{{$.Tests.Defaults.Env.AWS_REGION}}" {{- end}}),
			 Credentials: credentials.NewStaticCredentials(
							{{- if and (len $.testCase.LocalConfig) $.testCase.LocalConfig.AWS_ACCESS_KEY -}}
								"{{$.testCase.LocalConfig.AWS_ACCESS_KEY}}",							
							{{- else -}}
								"{{$.Tests.Defaults.Env.AWS_ACCESS_KEY}}",
							{{- end -}}

							{{- if and (len $.testCase.LocalConfig) $.testCase.LocalConfig.AWS_SECRET_ACCESS_KEY -}}
								"{{$.testCase.LocalConfig.AWS_SECRET_ACCESS_KEY}}",							
							{{- else -}}
								"{{$.Tests.Defaults.Env.AWS_SECRET_ACCESS_KEY}}",
							{{- end -}} ""),
		   }))
{{end}}

{{define "ResponseBuild"}}
		{{- if eq $.testCase.Response.StatusCode 0}}
			r.HTTPResponse = &http.Response{StatusCode:200,
											Header: http.Header{},
											Body: ioutil.NopCloser(&bytes.Buffer{}),
											}
		{{- else }}
			r.HTTPResponse = &http.Response{
							StatusCode:{{$.testCase.Response.StatusCode}},
						{{- if ne (len $.testCase.Response.Headers) 0}}
							Header: http.Header{
										{{- range $key,$val:=$.testCase.Response.Headers}}
											"{{$key}}":[]string{ "{{$val}}" },
										{{- end}}	
									},
						{{- else}}
							Header: http.Header{},
						{{- end}}

						{{- if ne (len $.testCase.Response.BodyContent) 0}}
							Body: ioutil.NopCloser(bytes.NewBufferString({{printf "%q" $.testCase.Response.BodyContent}})),
						{{- else}}
							Body: ioutil.NopCloser(&bytes.Buffer{}),
						{{- end}}
						}
		{{- end}}
{{end}}

{{define "RequestBuild"}}
		input := {{ $.testCase.Request.BuildInputShape $.op.InputRef }}

		//Build request
		req, resp := svc.{{$.testCase.Request.Operation}}Request(input)
		_ = resp

		MockHTTPResponseHandler := request.NamedHandler{Name: "core.SendHandler", Fn: func (r *request.Request){ 
			{{- template "ResponseBuild" Map "testCase" $.testCase -}}	
		}}
		req.Handlers.Send.Swap( corehandlers.SendHandler.Name, MockHTTPResponseHandler )

		err := req.Send()
		if err != nil {
			t.Errorf("expect no error, got %v", err)
		}
		{{printf "\n"}}
{{end}}

{{- range $i, $testCase := $.Tests.Cases }}
	//{{printf "%s" $testCase.Description}}
	{{- $op := index $.API.Operations $testCase.Request.Operation }}
	func TestBehavior_{{ printf "%02d" $i }}(t *testing.T) {

		{{template "StashCredentials" .}}

		{{- template "SessionSetup" Map "testCase" $testCase "Tests" $.Tests}}

		//Starts a new service using using sess
		svc := {{$.API.PackageName}}.New(sess)

		{{- template "RequestBuild" Map "testCase" $testCase "op" $op}}
		
		{{$testCase.GenerateAssertions $op}}

	}
{{- end }}
`))
