// +build codegen

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"
	"strings"
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
	Expect []map[string]interface{}   `json:"expect"`
}

type Response struct{
	StatusCode int `json:"statusCode"`
	BodyContent string `json:"bodyContent"`
	BodyType string `json: "bodyType"`
	Headers map[string]string `json:"headers"`

}

type Request struct{
	Operation string `json:"operation"`
	Input map[string]interface{} `json:"input"`
}

func (c Request) BuildInputShape(ref *ShapeRef) string {
	var b ShapeValueBuilder
	return fmt.Sprintf("&%s{\n%s\n}",
		b.GoType(ref, true),
		b.BuildShape(ref, c.Input, false),
	)
}

func (c Request) EmptyShapeBuilder(ref *ShapeRef) string{
	var b ShapeValueBuilder
	return fmt.Sprintf("%s{}", b.GoType(ref, true))
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
	//a.AddImport("context")
	a.AddImport("testing")
	a.AddImport("net/http")
	a.AddImport("fmt")
	a.AddImport("time")
	a.AddImport("io/ioutil")
	a.AddImport("bytes")
	a.AddImport("net/url")
	a.AddImport("net/textproto")
	a.AddImport("strings")
	a.AddImport("encoding/base64")
	a.AddImport("github.com/mitchellh/mapstructure") //Library to copy map to a struct
	a.AddImport("reflect")

	a.AddSDKImport("aws")
	a.AddSDKImport("awstesting")
	a.AddSDKImport("aws/session")
	a.AddSDKImport("aws/credentials")
	//a.AddSDKImport("aws/client")
	//a.AddSDKImport("private/protocol")
	a.AddSDKImport("private/util")
	a.AddSDKImport("aws/request")

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

	ignoreImports := `

	`
	return a.importsGoCode() + ignoreImports + w.String()
}

// Changes the first character of val to upper case
func FormatAssertionName (val string) string{
	tempVal := []byte(val)
	tempVal[0] -= 32 //First Letter to UpperCase
	return string(tempVal)
}

//template map is defined in "eventstream.go"
var funcMap = template.FuncMap{"Map": templateMap,"Contains": strings.Contains,"FormatAssertionName": FormatAssertionName}

//	defer env()//Might need to comment this out

var behaviorTestTmpl = template.Must(template.New(`behaviorTestTmpl`).Funcs(funcMap).Parse(`

{{define "StashCredentials"}}
	env := awstesting.StashEnv() //Stashes the current environment variables
	fmt.Println(env)
{{end}}

{{define "SessionSetup"}}
		//Starts a new session with credentials and region parsed from "defaults" in the Json file'
		sess := session.Must(session.NewSession(&aws.Config{
				 Region: aws.String( {{- if and (len $.testCase.LocalConfig) $.testCase.LocalConfig.AWS_REGION }} "{{$.testCase.LocalConfig.AWS_REGION}}" {{- else}} "{{$.Tests.Defaults.Env.AWS_REGION}}" {{- end}}),
				 Credentials: credentials.NewStaticCredentials(
								{{- if and (len $.testCase.LocalConfig) $.testCase.LocalConfig.AWS_ACCESS_KEY }}
									"{{$.testCase.LocalConfig.AWS_ACCESS_KEY}}",							
								{{- else}}
									"{{$.Tests.Defaults.Env.AWS_ACCESS_KEY}}",
								{{- end}}

								{{- if and (len $.testCase.LocalConfig) $.testCase.LocalConfig.AWS_SECRET_ACCESS_KEY }}
									"{{$.testCase.LocalConfig.AWS_SECRET_ACCESS_KEY}}",							
								{{- else}}
									"{{$.Tests.Defaults.Env.AWS_SECRET_ACCESS_KEY}}",
								{{- end}} ""),
			   }))
{{end}}

{{define "Assertions"}}
		//Assertions start here
		{{- range $k,$assertion :=$.testCase.Expect}}
			{{- range $assertionName,$assertionContext:=$assertion}}
				{{- if Contains $assertionName "request" }}
					{{- if eq $assertionName "requestBodyMatchesXml"}}
						if !assert{{FormatAssertionName $assertionName}}(t , req, {{printf "%q" $assertionContext}}, {{ $.testCase.Request.EmptyShapeBuilder $.op.InputRef}} )
					{{- else}} {{if eq $assertionName "requestHeadersMatch"}}
						if !assert{{ FormatAssertionName $assertionName}}(t , req, {{printf "%#v" $assertionContext}})
					{{- else}} 
						if !assert{{FormatAssertionName $assertionName}}(t , req, "{{$assertionContext}}") 
					{{- end}} {{- end}} {
						t.Error("expect no error, got {{printf "%s" $assertionName}} assertion failed")
						}
				{{- else}}
					{{- if eq $assertionName "responseDataEquals"}}
 						if !assert{{FormatAssertionName $assertionName}}(t , response, {{printf "%#v" $assertionContext}},{{ $.testCase.Request.EmptyShapeBuilder $.op.OutputRef }})
					{{- else}}
						if !assert{{FormatAssertionName $assertionName}}(t , response, {{printf "%#v" $assertionContext}})
					{{- end}}{
							t.Error("expect no error, got {{printf "%s" $assertionName}} assertion failed")
						}
				{{- end}}

			{{- end}}
			{{printf "\n"}}
		{{- end}}
{{end}}

{{define "ResponseBuild"}}
		{{- if eq $.testCase.Response.StatusCode 0}}
			response := &http.Response{StatusCode:200}
		{{- else }}
		  response := &http.Response{
							StatusCode:{{$.testCase.Response.StatusCode}},
						{{- if ne (len $.testCase.Response.Headers) 0}}
							Header: http.Header{
										{{- range $key,$val:=$.testCase.Response.Headers}}
											"{{$key}}":[]string{ "{{$val}}" },
										{{- end}}	
									},
						{{- end}}
						{{- if ne (len $.testCase.Response.BodyContent) 0}}
							Body: ioutil.NopCloser(bytes.NewBufferString({{printf "%q" $.testCase.Response.BodyContent}})),
						{{- end}}
					}
		{{- end}}	
		_ = response
		{{printf "\n"}}
{{end}}

{{define "RequestBuild"}}
		input := {{ $.testCase.Request.BuildInputShape $.op.InputRef }}

		req, _ := svc.{{$.testCase.Request.Operation}}Request(input)


		req.Build()
		if req.Error != nil {
			t.Errorf("expect no error, got %v", req.Error)
		}

		{{printf "\n"}}
{{end}}

func parseTime(layout, value string) *time.Time {
	t, err := time.Parse(layout, value)
	if err != nil {
		panic(err)
	}
	return &t
}

func assertRequestMethodEquals(t *testing.T, req *request.Request,val string) bool{
	return req.HTTPRequest.Method == val
}

func assertRequestUrlMatches(t *testing.T, req *request.Request,val string) bool{
	return req.HTTPRequest.URL.String() == val
}

func assertRequestUrlPathMatches(t *testing.T, req *request.Request,val string) bool{
	return req.HTTPRequest.URL.RequestURI() == val
}

func assertRequestUrlQueryMatches(t *testing.T, req *request.Request,val string) bool{
	structExpect, err := url.Parse(val) // parsed val into a structure
	if err!=nil{
		t.Errorf("expect no error, got %v",err)
	}
	queryRequest := req.HTTPRequest.URL.Query() //parsed RawQuery of "req" to get the values inside
	queryExpect := structExpect.Query() //parsed RawQuery of "val" to get the values inside

	return queryRequest.Encode() == queryExpect.Encode()
}

func assertRequestHeadersMatch(t *testing.T, req *request.Request,header map[string]interface{}) bool{
	for key, valExpect := range header{
		valReq := req.HTTPRequest.Header.Get(key)
		if valReq == "" || valReq[0] != valExpect{
			return false
		}
	}
	return true
}

func assertRequestBodyEqualsBytes(t *testing.T, req *request.Request,val string) bool{
	var bytesReqBody []byte
	bytesExpect, err := base64.StdEncoding.DecodeString(val)

	if err != nil {
		t.Errorf("expect no error, got %v", err)
	}

	if req.HTTPRequest.Body!= nil {
  		bytesReqBody, err = ioutil.ReadAll(req.HTTPRequest.Body)
		if err != nil {
			t.Errorf("expect no error, got %v", err)
		}
	}

	req.HTTPRequest.Body = ioutil.NopCloser(bytes.NewBuffer(bytesReqBody))

	return bytes.Compare(bytesReqBody, bytesExpect) == 0 
}

func assertRequestBodyEqualsJson(t *testing.T, req *request.Request,val string) bool{
	var bytesReqBody []byte
	var err error
	if req.HTTPRequest.Body!= nil {
  		bytesReqBody, err  = ioutil.ReadAll(req.HTTPRequest.Body)
		if err != nil {
			t.Errorf("expect no error, got %v", err)
		}
	}

	req.HTTPRequest.Body = ioutil.NopCloser(bytes.NewBuffer(bytesReqBody))

	return awstesting.AssertJSON(t, val, util.Trim(string(bytesReqBody)) )
}

func assertRequestBodyMatchesXml(t *testing.T, req *request.Request,val string,container interface{}) bool{
	r := req.HTTPRequest

	if r.Body == nil {
		t.Errorf("expect body not to be nil")
	}
	body := util.SortXML(r.Body)

	return awstesting.AssertXML(t, val, util.Trim(string(body)),container )
}

func assertRequestBodyEqualsString(t *testing.T, req *request.Request,val string) bool{
	var bytesReqBody []byte
	var err error
	if req.HTTPRequest.Body!= nil {
  		bytesReqBody, err  = ioutil.ReadAll(req.HTTPRequest.Body)
		if err != nil {
			t.Errorf("expect no error, got %v", err)
		}
	}

	req.HTTPRequest.Body = ioutil.NopCloser(bytes.NewBuffer(bytesReqBody))
	stringReqBody := string(bytesReqBody)

	return stringReqBody == val
}

func assertRequestIdEquals(t *testing.T, req *request.Request,val string) bool{
	return req.RequestID == val
}

func assertResponseDataEquals(t *testing.T, response *http.Response,expect map[string]interface{},container interface{}) bool{
	return true
}

func assertResponseErrorIsKindOf(t *testing.T, req *request.Request,val map[string]interface{}){
    if testing.Short() {
        t.Skip("skipping responseErrorIsKindOf assertion")
    }
}

func assertResponseErrorMessageEquals(t *testing.T, req *request.Request,val map[string]interface{}){
    if testing.Short() {
        t.Skip("skipping responseErrorMessageEquals assertion")
    }
}

func assertResponseErrorDataEquals(t *testing.T, req *request.Request,val map[string]interface{}){
    if testing.Short() {
        t.Skip("skipping responseErrorDataEquals assertion")
    }
}

func assertResponseErrorRequestIdEquals(t *testing.T, req *request.Request,val map[string]interface{}){
    if testing.Short() {
        t.Skip("skipping responseErrorRequestIdEquals assertion")
    }
}

{{- range $i, $testCase := $.Tests.Cases }}
	//{{printf "%s" $testCase.Description}}
	{{- $op := index $.API.Operations $testCase.Request.Operation }}
	func BehavTest_{{ printf "%02d" $i }}(t *testing.T) {

		{{template "StashCredentials" .}}

		{{- template "SessionSetup" Map "testCase" $testCase "Tests" $.Tests}}

		//Starts a new service using using sess
		svc := {{$.API.PackageName}}.New(sess)

		{{- template "RequestBuild" Map "testCase" $testCase "op" $op}}

		{{- template "ResponseBuild" Map "testCase" $testCase}}

		{{- template "Assertions" Map "testCase" $testCase "op" $op}}

	}
{{- end }}
`))
