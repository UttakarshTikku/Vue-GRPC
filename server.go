package main

import (
	library "github.com/UttakarshTikku/Vue-GRPC/_proto/example"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"os"
	"log"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"net/http"
	"fmt"
	"sync"
	"sync/atomic"
	"io/ioutil"
	"bytes"
	"encoding/json"
	"time"
	"math"
)

var (
	PORT = 9090

	// we use a map as an in-memory store for the application, it is protected by a RWMutes to allow
	// safe concurrent access.
	NZBN_RESPONSE_LOCK = &sync.RWMutex{}

	NZBN_RESPONSE_INDEX int64 = 1
	NZBN_RESPONSE = map[int64]*library.NZBNResponse {}
)

func main(){
	grpcServer := grpc.NewServer()
	library.RegisterNzbnServiceServer(grpcServer, &nzbnService{})
	grpclog.SetLogger(log.New(os.Stdout, "GRPC:", log.LstdFlags))

	wrappedServer := grpcweb.WrapServer(grpcServer)

	handler := func(res http.ResponseWriter, req *http.Request){
		wrappedServer.ServeHTTP(res, req)
	}

	httpServer := &http.Server{
		Addr: fmt.Sprintf(":%d", PORT),
		Handler: http.HandlerFunc(handler),
	}

	grpclog.Println("Starting server...")
	log.Fatalln(httpServer.ListenAndServe())
}

type nzbnService struct {}

func RestfulCallToNzbnValidator(queryString string) []byte {
	var buffer bytes.Buffer
	buffer.WriteString("https://sandbox.api.business.govt.nz/services/v3/nzbn/entities/")
	buffer.WriteString(queryString)

	req, _ := http.NewRequest("GET", buffer.String(), nil)

	req.Header.Add("Authorization", "Bearer d414c630b16ce64d6daf73ca89f83c23")
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Add("Postman-Token", "ed1d43b6-1ec0-ed51-730f-3d819822902f")

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	return body
}

func (ns *nzbnService) RequestNZBNCheck(ctx context.Context, req *library.NZBNRequest)(*library.NZBNResponse, error){
	t1 := time.Now()
	grpc.SendHeader(ctx, metadata.Pairs("Pre-Response-Metadata", "Is-sent-as-headers-unary"))

	grpclog.Println("In the Request NZBN Check Method")
	grpclog.Println(req)

	NZBN_RESPONSE_LOCK.Lock()
	defer NZBN_RESPONSE_LOCK.Unlock()

	atomic.AddInt64(&NZBN_RESPONSE_INDEX, 1)
	id := atomic.LoadInt64(&NZBN_RESPONSE_INDEX)

	grpclog.Println(id)

	t3 := time.Now()
	jsonString := RestfulCallToNzbnValidator(req.Nzbnnumber);
	t4 := time.Now()
	hs := t4.Sub(t3).Hours()
	hs, mf := math.Modf(hs)
	ms := mf * 60
	ms, sf := math.Modf(ms)
	ss := sf * 60
	fmt.Println(hs, "hours", ms, "minutes", ss, "seconds")

	var dat map[string]interface{}
	err := json.Unmarshal(jsonString, &dat)
	if err != nil {
			panic(err)
	}

	if ((dat != nil) && dat["registeredAddress"] != nil && dat["registeredAddress"].([]interface{})[0] != nil){
		var a = dat["registeredAddress"].([]interface{})[0].(map[string]interface{})

		registeredaddress := &library.RegisteredAddress {
		 	Address1 : a["address1"].(string),
		 	Address2 : a["address2"].(string),
			Address3 : a["address3"].(string),
			Countrycode : a["countryCode"].(string),
		}

		companydetails := &library.CompanyDetails {
			Entityname : dat["entityName"].(string),
	    Entitytypedescription : dat["entityTypeDescription"].(string),
			Entitystatusdescription : dat["entityStatusDescription"].(string),
	    Registrationdate : dat["registrationDate"].(string),
	    Address : registeredaddress,
		}

		response := &library.NZBNResponse{
			Id: id,
			Nzbnnumber: req.Nzbnnumber,
			Nzbnresponse: companydetails,
		}

		NZBN_RESPONSE[response.Id] = response
		t2 := time.Now()
		hs := t2.Sub(t1).Hours()
		hs, mf := math.Modf(hs)
		ms := mf * 60
		ms, sf := math.Modf(ms)
		ss := sf * 60
		fmt.Println(hs, "hours", ms, "minutes", ss, "seconds")
		return response, nil
	} else {
		return nil, grpc.Errorf(codes.NotFound, "Could not be found.")
	}
}

func (ns *nzbnService) GetHistory(ctx context.Context, req *library.GetHistoryRequest)(*library.GetHistoryResponse, error){
	grpc.SendHeader(ctx, metadata.Pairs("Pre-Response-Metadata", "Is-sent-as-headers-unary"))

	NZBN_RESPONSE_LOCK.RLock()
	defer NZBN_RESPONSE_LOCK.RUnlock()

	requests := make([]*library.NZBNResponse, len(NZBN_RESPONSE))

	idx := 0
	for _, v := range NZBN_RESPONSE {
		requests[idx] = v
		idx++
	}

	resp := &library.GetHistoryResponse{ Response: requests }
	return resp, nil
}

func (ns *nzbnService) DeleteHistory(ctx context.Context, req *library.DeleteHistoryRequest) (*library.NZBNResponse, error){
	grpc.SendHeader(ctx, metadata.Pairs("Pre-Response-Metadata", "Is-sent-as-headers-unary"))

	NZBN_RESPONSE_LOCK.Lock()
	defer NZBN_RESPONSE_LOCK.Unlock()

	if historyRequest, exists := NZBN_RESPONSE[req.Id]; exists {
		request := &library.NZBNResponse{
			Id: historyRequest.Id,
			Nzbnnumber: historyRequest.Nzbnnumber,
			Nzbnresponse: historyRequest.Nzbnresponse,
		}

		delete(NZBN_RESPONSE, req.Id)
		grpclog.Println(request)
		return request, nil
	}
	grpclog.Println(grpc.Errorf(codes.NotFound, "Could not be found."))
	return nil, grpc.Errorf(codes.NotFound, "Could not be found.")
}
