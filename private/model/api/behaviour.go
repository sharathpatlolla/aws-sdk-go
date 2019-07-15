// +build codegen

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"
)

type BehaviourTestSuite struct {
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
	Expect []map[string]interface{}   `json:"expect"`
}

type Request struct{
	Operation string `json:"operation"`
	Input map[string]interface{} `json:"input"`
}

// AttachBehaviourTests attaches the Behaviour test cases to the API model.
func (a *API) AttachBehaviourTests(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		panic(fmt.Sprintf("failed to open behaviour tests %s, err: %v", filename, err))
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&a.BehaviourTests); err != nil {
		panic(fmt.Sprintf("failed to decode behaviour tests %s, err: %v", filename, err))
	}

}

// APIBehaviourTestsGoCode returns the Go Code string for the Behaviour tests.
func (a *API) APIBehaviourTestsGoCode() string {
	w := bytes.NewBuffer(nil)
	a.resetImports()
	a.AddImport("context")
	a.AddImport("testing")
	a.AddImport("time")
	a.AddSDKImport("aws")
	a.AddSDKImport("awstesting")
	a.AddSDKImport("aws/client")
	a.AddSDKImport("private/protocol")
	a.AddSDKImport("private/util")
	a.AddSDKImport("aws/request")


	a.AddImport(a.ImportPath())

	behaviourTests := struct {
		API *API
		BehaviourTestSuite
	}{
		API:            a,
		BehaviourTestSuite: a.BehaviourTests,
	}

	if err := behaviourTestTmpl.Execute(w, behaviourTests); err != nil {
		panic(fmt.Sprintf("failed to create behaviour tests, %v", err))
	}


	return a.importsGoCode() + w.String()
}

var funcMap = template.FuncMap{"Map": templateMap}

var behaviourTestTmpl = template.Must(template.New(`behaviourTestTmpl`).Funcs(funcMap).Parse(`

{{define "StashCredentials"}}
	env := awstesting.StashEnv() //Stashes the current environment variables
{{end}}

{{define "SessionSetup"}}
		{{- if len $.testCase.LocalConfig }}
			access_key="{{$.testCase.LocalConfig.AWS_ACCESS_KEY}}"
			secret_access_key="{{$.testCase.LocalConfig.AWS_SECRET_ACCESS_KEY}}"
			aws_region="{{$.testCase.LocalConfig.AWS_REGION}}"
		{{- else}}
			access_key:="{{$.Tests.Defaults.Env.AWS_ACCESS_KEY}}"
			secret_access_key:="{{$.Tests.Defaults.Env.AWS_SECRET_ACCESS_KEY}}"
			aws_region:="{{$.Tests.Defaults.Env.AWS_REGION}}"
		{{- end}}

		//Starts a new session with credentials and region parsed from "defaults" in the Json file'
		sess := session.Must(session.NewSession(&aws.Config{
				 Region: aws.String(aws_region),
				 Credentials: credentials.NewStaticCredentials(access_key, secret_access_key, ""),
			   }))
{{end}}

{{- range $i, $testCase := $.Tests.Cases }}
	//Client for BehavTest_{{ printf "%02d" $i }}
	type BehavTestClient_{{ printf "%02d" $i }} struct {
		*client.Client
	}

	//Output for request of BehavTest_{{ printf "%02d" $i }}
	type BehavTestRequestOutput_{{ printf "%02d" $i }} struct {
		_ struct{} 
	}
	//Generates request for BehavTest_{{ printf "%02d" $i }}
	func (c *BehavTestClient_{{ printf "%02d" $i }}) BehavTestRequestGenerator_{{printf "%02d" $i }}(input string )(req *request.Request, output string ){
		op := &request.Operation{
			Name:       "{{$testCase.Request.Operation}}",
		{{- range $j, $expects := $testCase.Expect }}
			{{- if eq $j 0 }}
				HTTPMethod: "{{$expects.requestMethodEquals}}",
			{{end}}
		{{end}}
		}
		output = &BehavTestRequestOutput_{}
		req := c.NewRequest(op, input, output)
		req.Handlers.Unmarshal.Swap(restjson.UnmarshalHandler.Name, protocol.UnmarshalDiscardBodyHandler)
		return
	}

	//{{printf "%s" $testCase.Description}}
	func BehavTest_{{ printf "%02d" $i }}(t *testing.T) {

		{{template "StashCredentials" .}}
		{{- template "SessionSetup" Map "testCase" $testCase "Tests" $.Tests}}
		
		//Starts a new service using using sess
		svc := {{$.API.PackageName}}.New(sess)

		req, _ := svc.BehavTestRequestGenerator_{{printf "%02d" $i }}("")
		r := req.HTTPRequest
	
		// build request
		req.Build()

		fmt.Println("Write behaviour tests here")
	}
{{- end }}
`))

