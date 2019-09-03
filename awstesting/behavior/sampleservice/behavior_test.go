// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

// +build go1.9

package sampleservice_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/corehandlers"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/awstesting"
	"github.com/aws/aws-sdk-go/awstesting/behavior/sampleservice"
	"github.com/aws/aws-sdk-go/internal/sdktesting"
	"github.com/aws/aws-sdk-go/private/protocol"
)

var _ *time.Time
var _ = protocol.ParseTime
var _ = strings.NewReader
var _ = json.Marshal

// Prefers configured region over ENV region
func TestBehavior_000(t *testing.T) {

	restoreEnv := sdktesting.StashEnv() //Stashes the current environment
	defer restoreEnv()

	// Starts a new session with credentials and region parsed from "defaults" in the Json file'
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String("us-west-1"),
		Credentials: credentials.NewStaticCredentials("akid", "secret", ""),
	}))

	//Starts a new service using using sess
	svc := sampleservice.New(sess)

	input := &sampleservice.EmptyOperationInput{}

	//Build request
	req, resp := svc.EmptyOperationRequest(input)
	_ = resp

	MockHTTPResponseHandler := request.NamedHandler{Name: "core.SendHandler", Fn: func(r *request.Request) {

		r.HTTPResponse = &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
			Body:       ioutil.NopCloser(&bytes.Buffer{}),
		}
	}}
	req.Handlers.Send.Swap(corehandlers.SendHandler.Name, MockHTTPResponseHandler)

	err := req.Send()
	if err != nil {
		t.Fatal(err)
	}

	//Assertions start here
	awstesting.AssertRequestURLMatches(t, "https://sample-service.us-west-1.amazonaws.com/", req.HTTPRequest.URL.String())

}
