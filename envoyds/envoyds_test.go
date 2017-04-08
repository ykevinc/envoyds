package main

import (
	"bytes"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"github.com/ykevinc/envoyds"
	"net/http"
	"strconv"
	"testing"
	"time"
)

const (
	TEST_HOST           = "localhost"
	TEST_REDIS_PORT     = 6379
	TEST_HTTP_PORT      = 8000
	TEST_SERVICE_PREFIX = "test_integration_service"
)

func setUp(t *testing.T) {
	var err error
	r, err := envoyds.NewRouter("test", TEST_HOST, TEST_REDIS_PORT)
	if err != nil {
		t.Fatal(err)
	}
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", TEST_HTTP_PORT),
		Handler: r,
	}
	go server.ListenAndServe()
}

func tearDown(t *testing.T, testService string) {
	getResponse := envoyds.ServiceGetResponse{}
	marshaler := jsonpb.Marshaler{EmitDefaults: true, OrigName: true}
	callToGet(t, &marshaler, &getResponse, testService)
	if len(getResponse.Hosts) != 0 {
		t.Fatalf("tear down: got %d hosts, want %d hosts", len(getResponse.Hosts), 0)
	}
}

func testRegister(t *testing.T, testService string) {
	postRequestPort33 := envoyds.ServicePostRequest{}
	postRequestPort33.Port = 33
	postRequestPort33.Ip = "123.123.123.123"
	postRequestPort33.Tags = &envoyds.Tags{Az: "az33"}
	postRequestPort36 := envoyds.ServicePostRequest{}
	postRequestPort36.Port = 36
	postRequestPort36.Ip = "123.123.123.123"
	postRequestPort33.Tags = &envoyds.Tags{Az: "az36"}

	inputs := []*envoyds.ServicePostRequest{
		&postRequestPort33,
		&postRequestPort33,
		&postRequestPort36,
	}

	marshaler := jsonpb.Marshaler{EmitDefaults: true, OrigName: true}
	for _, input := range inputs {
		callToRegister(t, &marshaler, input, testService, http.StatusOK)
	}

	getResponse := envoyds.ServiceGetResponse{}
	callToGet(t, &marshaler, &getResponse, testService)

	if len(getResponse.Hosts) != 2 {
		t.Fatalf("got %d hosts, want %d hosts", len(getResponse.Hosts), 2)
	}

	defer callToDelete(t, testService, postRequestPort33.GetIp(), int(postRequestPort33.GetPort()), http.StatusOK)
	defer callToDelete(t, testService, postRequestPort36.GetIp(), int(postRequestPort36.GetPort()), http.StatusOK)
}

func testDelete(t *testing.T, testService string) {
	postRequestPort33 := envoyds.ServicePostRequest{}
	postRequestPort33.Port = 33
	postRequestPort33.Ip = "123.123.123.123"
	postRequestPort33.Tags = &envoyds.Tags{Az: "az33"}
	postRequestPort36 := envoyds.ServicePostRequest{}
	postRequestPort36.Port = 36
	postRequestPort36.Ip = "123.123.123.123"
	postRequestPort33.Tags = &envoyds.Tags{Az: "az36"}

	inputs := []*envoyds.ServicePostRequest{
		&postRequestPort33,
		&postRequestPort33,
		&postRequestPort36,
	}

	marshaler := jsonpb.Marshaler{EmitDefaults: true, OrigName: true}
	for _, input := range inputs {
		callToRegister(t, &marshaler, input, testService, http.StatusOK)
	}

	defer callToDelete(t, testService, postRequestPort33.GetIp(), int(postRequestPort33.GetPort()), http.StatusOK)
	defer callToDelete(t, testService, postRequestPort36.GetIp(), int(postRequestPort36.GetPort()), http.StatusBadRequest)
	defer callToDelete(t, testService, postRequestPort36.GetIp(), int(postRequestPort36.GetPort()), http.StatusOK)

	getResponse := envoyds.ServiceGetResponse{}
	callToGet(t, &marshaler, &getResponse, testService)
	if len(getResponse.Hosts) != 2 {
		t.Fatalf("got %d hosts, want %d hosts", len(getResponse.Hosts), 2)
	}
}

func testUpdate(t *testing.T, testService string) {
	oldWeight := int32(78)
	newWeight := int32(3)

	postRequestPort33 := envoyds.ServicePostRequest{}
	postRequestPort33.Port = 37
	postRequestPort33.Ip = "123.123.123.127"
	postRequestPort33.Tags = &envoyds.Tags{LoadBalancingWeight: oldWeight}
	marshaler := jsonpb.Marshaler{EmitDefaults: true, OrigName: true}
	callToRegister(t, &marshaler, &postRequestPort33, testService, http.StatusOK)
	defer callToDelete(t, testService, postRequestPort33.GetIp(), int(postRequestPort33.GetPort()), http.StatusOK)

	getResponse := envoyds.ServiceGetResponse{}
	callToGet(t, &marshaler, &getResponse, testService)

	if len(getResponse.Hosts) != 1 {
		t.Fatalf("got %d hosts, want %d hosts", len(getResponse.Hosts), 1)
	}

	if getResponse.Hosts[0].Tags.LoadBalancingWeight != oldWeight {
		t.Fatalf("got weight %d, want weight %d", getResponse.Hosts[0].Tags.LoadBalancingWeight, oldWeight)
	}
	callToUpdateWeight(t, &marshaler, testService, postRequestPort33.GetIp(), int(postRequestPort33.GetPort()), newWeight, http.StatusOK)
	callToGet(t, &marshaler, &getResponse, testService)
	if len(getResponse.Hosts) != 1 {
		t.Fatalf("got %d hosts, want %d hosts", len(getResponse.Hosts), 1)
	}
	if getResponse.Hosts[0].Tags.LoadBalancingWeight != newWeight {
		t.Fatalf("got weight %d, want weight %d", getResponse.Hosts[0].Tags.LoadBalancingWeight, newWeight)
	}
}

func callToRegister(t *testing.T, marshaler *jsonpb.Marshaler, postRequest *envoyds.ServicePostRequest, testService string, expectHttpStatus int) {
	v, err := marshaler.MarshalToString(postRequest)
	if err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewBufferString(v)
	resp, err := http.Post(fmt.Sprintf("http://%s:%d/v1/registration/%s", TEST_HOST, TEST_HTTP_PORT, testService), "application/json; charset=utf-8", reader)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != expectHttpStatus {
		t.Fatal(resp.StatusCode)
	}
}

func callToGet(t *testing.T, marshaler *jsonpb.Marshaler, getResponse *envoyds.ServiceGetResponse, testService string) {
	resp, err := http.Get(fmt.Sprintf("http://%s:%d/v1/registration/%s", TEST_HOST, TEST_HTTP_PORT, testService))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := jsonpb.Unmarshal(resp.Body, getResponse); err != nil {
		t.Fatal(err)
	}
}

func callToDelete(t *testing.T, testService string, ip string, port int, expectHttpStatus int) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://%s:%d/v1/registration/%s/%s/%d", TEST_HOST, TEST_HTTP_PORT, testService, ip, port), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != expectHttpStatus {
		t.Fatal(resp.StatusCode)
	}
}

func callToUpdateWeight(t *testing.T, marshaler *jsonpb.Marshaler, testService string, ip string, port int, weight int32, expectHttpStatus int) {
	var postRequest envoyds.ServiceUpdateLoadBalancingRequest
	postRequest.LoadBalancingWeight = weight
	v, err := marshaler.MarshalToString(&postRequest)
	if err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewBufferString(v)
	resp, err := http.Post(fmt.Sprintf("http://%s:%d/v1/loadbalancing/%s/%s/%d", TEST_HOST, TEST_HTTP_PORT, testService, ip, port), "application/json; charset=utf-8", reader)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectHttpStatus {
		t.Fatal(resp.StatusCode)
	}
}

func TestServer(t *testing.T) {
	testService := TEST_SERVICE_PREFIX + strconv.Itoa(int(time.Now().Unix()))
	t.Log("test serviceName=" + testService)
	setUp(t)
	defer tearDown(t, testService)
	testRegister(t, testService)
	testDelete(t, testService)
	testUpdate(t, testService)
}
