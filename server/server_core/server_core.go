/**
* (C) 2020 Mitsubishi Electrics Automotive
* (C) 2019 Geotab Inc
* (C) 2019 Volvo Cars
*
* All files and artifacts in the repository at https://github.com/MEAE-GOT/W3C_VehicleSignalInterfaceImpl
* are licensed under the provisions of the license provided by the LICENSE file in this repository.
*
**/

package main

import (
	 //   "fmt"
	"flag"
	"regexp"

	"github.com/gorilla/websocket"

	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
        "sort"
	"unsafe"

	"github.com/MEAE-GOT/W3C_VehicleSignalInterfaceImpl/utils"
)

// #include <stdlib.h>
// #include <stdio.h>
// #include <stdbool.h>
// #include "vssparserutilities.h"
import "C"

var VSSTreeRoot C.long

type searchData_t struct { // searchData_t defined in vssparserutilities.h
	responsePath    [512]byte // vssparserutilities.h: #define MAXCHARSPATH 512; typedef char path_t[MAXCHARSPATH];
	foundNodeHandle int64     // defined as long in vssparserutilities.h
}

type filterDef_t struct {
	name     string
	operator string
	value    string
}

var transportRegChan chan int
var transportRegPortNum int = 8081
var transportDataPortNum int = 8100 // port number interval [8100-]

// add element to both channels if support for new transport protocol is added
var transportDataChan = []chan string{
	make(chan string),
	make(chan string),
}

var backendChan = []chan string{
	make(chan string),
	make(chan string),
}

/*
* To add support for one more transport manager protocol:
*    - add a map entry to supportedProtocols
*    - add a komponent to the muxServer array
*    - add a component to the transportDataChan array
*    - add a select case in the main loop
 */
var supportedProtocols = map[int]string{
	0: "HTTP",
	1: "WebSocket",
}

var serviceRegChan chan string
var serviceRegPortNum int = 8082
var serviceDataPortNum int = 8200 // port number interval [8200-]

// add element if support for new service manager is added
var serviceDataChan = []chan string{
	make(chan string),
	make(chan string),
}

/** muxServer[0] is assigned to transport registration server,
*   muxServer[1] is assigned to service registration server,
*   of the following the first half is assigned for transport data servers,
*   and the second half is assigned for service data clients
**/
var muxServer = []*http.ServeMux{
	http.NewServeMux(), // 0 = transport reg
	http.NewServeMux(), // 1 = service reg
	http.NewServeMux(), // 2 = transport data
	http.NewServeMux(), // 3 = transport data
	http.NewServeMux(), // 4 = service data
	http.NewServeMux(), // 5 = service data
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type RouterTable_t struct {
	mgrId    int
	mgrIndex int
}

var routerTable []RouterTable_t

var errorResponseMap = map[string]interface{}{
	"MgrId":     0,
	"ClientId":  0,
	"action":    "unknown",
	"requestId": "XXX",
	"error":     `{"number":AAA, "reason": "BBB", "message": "CCC"}`,
	"timestamp": 1234,
}

/*
* Core-server main tasks:
    - server for transportmgr registrations
    - server for servicemgr registrations
    - server in transportmgr data channel requests
    - client in servicemgr data channel requests
    - router hub for request-response messages
    - request message path verification
    - request message access restriction control
    - service discovery response synthesis
*/

func routerTableAdd(mgrId int, mgrIndex int) {
	var tableElement RouterTable_t
	tableElement.mgrId = mgrId
	tableElement.mgrIndex = mgrIndex
	routerTable = append(routerTable, tableElement)
}

func routerTableSearchForMgrIndex(mgrId int) int {
	for _, element := range routerTable {
		if element.mgrId == mgrId {
			utils.Info.Printf("routerTableSearchForMgrIndex: Found index=%d", element.mgrIndex)
			return element.mgrIndex
		}
	}
	return -1
}

func getPayloadMgrId(request string) int {
	type Payload struct {
		MgrId int
	}
	decoder := json.NewDecoder(strings.NewReader(request))
	var payload Payload
	err := decoder.Decode(&payload)
	if err != nil {
		utils.Error.Printf("Server core-getPayloadMgrId: JSON decode failed for request:%s\n", request)
		return -1
	}
	return payload.MgrId
}

/*
* The transportRegisterServer assigns a requesting transport mgr the data channel port number to use,
* the data channel URL path, and the transport mgr ID that shall be added to the server internal req/resp messages.
* This is communicated to the coreserver that will save it in its router database.
* The port number returned is unique per protocol supported.
* If there is a need to support registering of multiple mgrs for the same protocol,
* then caching assigned mgr data can be used to assign other unique portno + mgr ID. Currently not supported.
 */
func maketransportRegisterHandler(transportRegChannel chan int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		utils.Info.Printf("transportRegisterServer():url=%s", req.URL.Path)
		mgrIndex := -1
		if req.URL.Path != "/transport/reg" {
			http.Error(w, "404 url path not found.", 404)
		} else if req.Method != "POST" {
			http.Error(w, "400 bad request method.", 400)
		} else {
			type Payload struct {
				Protocol string
			}
			decoder := json.NewDecoder(req.Body)
			var payload Payload
			err := decoder.Decode(&payload)
			if err != nil {
				panic(err)
			}
			utils.Info.Printf("transportRegisterServer():POST request=%s", payload.Protocol)
			for key, value := range supportedProtocols {
				if payload.Protocol == value {
					mgrIndex = key
				}
			}
			if mgrIndex != -1 { // communicate: port no + mgr Id to server hub, port no + url path + mgr Id to transport mgr
				transportRegChannel <- transportDataPortNum + mgrIndex // port no
				mgrId := rand.Intn(65535)                              // [0 -65535], 16-bit value
				transportRegChannel <- mgrId                           // mgr id
				w.Header().Set("Content-Type", "application/json")
				response := "{ \"Portnum\" : " + strconv.Itoa(transportDataPortNum+mgrIndex) + " , \"Urlpath\" : \"/transport/data/" + strconv.Itoa(mgrIndex) + "\"" + " , \"Mgrid\" : " + strconv.Itoa(mgrId) + " }"

				utils.Info.Printf("transportRegisterServer():POST response=%s", response)
				w.Write([]byte(response)) // correct JSON?
				routerTableAdd(mgrId, mgrIndex)
			} else {
				http.Error(w, "404 protocol not supported.", 404)
			}
		}
	}
}

func initTransportRegisterServer(transportRegChannel chan int) {
	utils.Info.Printf("initTransportRegisterServer(): :8081/transport/reg")
	transportRegisterHandler := maketransportRegisterHandler(transportRegChannel)
	muxServer[0].HandleFunc("/transport/reg", transportRegisterHandler)
	utils.Error.Fatal(http.ListenAndServe(":8081", muxServer[0]))
}

func frontendServiceDataComm(dataConn *websocket.Conn, request string) {
	err := dataConn.WriteMessage(websocket.TextMessage, []byte(request))
	if err != nil {
		utils.Error.Print("Service datachannel write error:", err)
	}
}

func backendServiceDataComm(dataConn *websocket.Conn, backendChannel []chan string, serviceIndex int) {
	for {
		_, response, err := dataConn.ReadMessage()
		utils.Info.Printf("Server core: Response from service mgr:%s", string(response))
		var responseMap = make(map[string]interface{})
		if err != nil {
			utils.Error.Println("Service datachannel read error:", err)
			response = []byte(utils.FinalizeMessage(errorResponseMap)) // needs improvement
		} else {
			utils.ExtractPayload(string(response), &responseMap)
		}
		if responseMap["action"] == "subscription" {
			mgrIndex := routerTableSearchForMgrIndex(int(responseMap["MgrId"].(float64)))
			backendChannel[mgrIndex] <- string(response)
		} else {
			serviceDataChan[serviceIndex] <- string(response) // response to request
		}
	}
}

/**
* initServiceDataSession:
* sets up the WS based communication (as client) with a service manager
**/
func initServiceDataSession(muxServer *http.ServeMux, serviceIndex int, backendChannel []chan string, remoteIp string) (dataConn *websocket.Conn) {
	var addr = flag.String("addr", remoteIp+":"+strconv.Itoa(serviceDataPortNum+serviceIndex), "http service address")
	dataSessionUrl := url.URL{Scheme: "ws", Host: *addr, Path: "/service/data/" + strconv.Itoa(serviceIndex)}
	utils.Info.Printf("Connecting to:%s", dataSessionUrl.String())
	dataConn, _, err := websocket.DefaultDialer.Dial(dataSessionUrl.String(), http.Header{"Access-Control-Allow-Origin": {"*"}})
	//    dataConn, _, err := websocket.DefaultDialer.Dial(dataSessionUrl.String(), nil)
	if err != nil {
		utils.Error.Fatal("Service data session dial error:", err)
		return nil
	}
	go backendServiceDataComm(dataConn, backendChannel, serviceIndex)
	return dataConn
}

func initServiceClientSession(serviceDataChannel chan string, serviceIndex int, backendChannel []chan string, remoteIp string) {
	time.Sleep(3 * time.Second)                               //wait for service data server to be initiated (initiate at first app-client request instead...)
	muxIndex := (len(muxServer)-2)/2 + 1 + (serviceIndex + 1) //could be more intuitive...
	utils.Info.Printf("initServiceClientSession: muxIndex=%d", muxIndex)
	dataConn := initServiceDataSession(muxServer[muxIndex], serviceIndex, backendChannel, remoteIp)
	for {
		select {
		case request := <-serviceDataChannel:
			frontendServiceDataComm(dataConn, request)
		}
	}
}

func makeServiceRegisterHandler(serviceRegChannel chan string, serviceIndex *int, backendChannel []chan string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		var re = regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`)
		remoteIp := re.FindString(req.RemoteAddr)
		utils.Info.Printf("serviceRegisterServer():remoteIp=%s, path=%s", remoteIp, req.URL.Path)
		if req.URL.Path != "/service/reg" {
			http.Error(w, "404 url path not found.", 404)
		} else if req.Method != "POST" {
			http.Error(w, "400 bad request method.", 400)
		} else {
			type Payload struct {
				Rootnode string
			}
			decoder := json.NewDecoder(req.Body)
			var payload Payload
			err := decoder.Decode(&payload)
			if err != nil {
				panic(err)
			}
			utils.Info.Printf("serviceRegisterServer(index=%d):received POST request=%s", *serviceIndex, payload.Rootnode)
			if *serviceIndex < 2 { // communicate: port no + root node to server hub, port no + url path to transport mgr, and start a client session
				serviceRegChannel <- strconv.Itoa(serviceDataPortNum + *serviceIndex)
				serviceRegChannel <- payload.Rootnode
				*serviceIndex += 1
				w.Header().Set("Content-Type", "application/json")
				response := "{ \"Portnum\" : " + strconv.Itoa(serviceDataPortNum+*serviceIndex-1) + " , \"Urlpath\" : \"/service/data/" + strconv.Itoa(*serviceIndex-1) + "\"" + " }"

				utils.Info.Printf("serviceRegisterServer():POST response=%s", response)
				w.Write([]byte(response))
				go initServiceClientSession(serviceDataChan[*serviceIndex-1], *serviceIndex-1, backendChannel, remoteIp)
			} else {
				utils.Info.Printf("serviceRegisterServer():Max number of services already registered.")
			}
		}
	}
}

func initServiceRegisterServer(serviceRegChannel chan string, serviceIndex *int, backendChannel []chan string) {
	utils.Info.Printf("initServiceRegisterServer(): :8082/service/reg")
	serviceRegisterHandler := makeServiceRegisterHandler(serviceRegChannel, serviceIndex, backendChannel)
	muxServer[1].HandleFunc("/service/reg", serviceRegisterHandler)
	utils.Error.Fatal(http.ListenAndServe(":8082", muxServer[1]))
}

func frontendWSDataSession(conn *websocket.Conn, transportDataChannel chan string, backendChannel chan string) {
	defer conn.Close()
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			utils.Error.Print("read error data WS protocol.", err)
			break
		}

		utils.Info.Printf("%s request: %s", conn.RemoteAddr(), string(msg))
		transportDataChannel <- string(msg) // send request to server hub
		response := <-transportDataChannel  // wait for response from server hub

		backendChannel <- response
	}
}

func backendWSDataSession(conn *websocket.Conn, backendChannel chan string) {
	defer conn.Close()
	for {
		message := <-backendChannel

		utils.Info.Printf("%s Transport mgr server: message= %s", conn.RemoteAddr(), message)
		err := conn.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			utils.Error.Print("write error data WS protocol.", err)
			break
		}
	}
}

func makeTransportDataHandler(transportDataChannel chan string, backendChannel chan string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Upgrade") == "websocket" {
			utils.Info.Printf("we are upgrading to a websocket connection.")
			upgrader.CheckOrigin = func(r *http.Request) bool { return true }
			conn, err := upgrader.Upgrade(w, req, nil)
			if err != nil {
				utils.Error.Print("upgrade:", err)
				return
			}
			utils.Info.Printf("WS data session initiated.")
			go frontendWSDataSession(conn, transportDataChannel, backendChannel)
			go backendWSDataSession(conn, backendChannel)
		} else {
			http.Error(w, "400 protocol must be websocket.", 400)
		}
	}
}

/**
*  All transport data servers implement a WS server which communicates with a transport protocol manager.
**/
func initTransportDataServer(mgrIndex int, muxServer *http.ServeMux, transportDataChannel []chan string, backendChannel []chan string) {
	utils.Info.Printf("initTransportDataServer():mgrIndex=%d", mgrIndex)
	transportDataHandler := makeTransportDataHandler(transportDataChannel[mgrIndex], backendChannel[mgrIndex])
	muxServer.HandleFunc("/transport/data/"+strconv.Itoa(mgrIndex), transportDataHandler)
	utils.Error.Fatal(http.ListenAndServe(":"+strconv.Itoa(transportDataPortNum+mgrIndex), muxServer))
}

func initTransportDataServers(transportDataChannel []chan string, backendChannel []chan string) {
	for key, _ := range supportedProtocols {
		go initTransportDataServer(key, muxServer[key+2], transportDataChannel, backendChannel) //muxelements 0 and one assigned to reg servers
	}
}

func updateServiceRouting(portNo string, rootNode string) {
	utils.Info.Printf("updateServiceRouting(): portnum=%s, rootNode=%s", portNo, rootNode)
}

func initVssFile() bool {
	filePath := "vss_gen2.cnative"
	cfilePath := C.CString(filePath)
	VSSTreeRoot = C.VSSReadTree(cfilePath)
	C.free(unsafe.Pointer(cfilePath))

	if VSSTreeRoot == 0 {
		//		utils.Error.Println("Tree file not found")
		return false
	}

	return true
}

func searchTree(rootNode C.long, path string, searchData *searchData_t, anyDepth C.bool, leafNodesOnly C.bool, validation *C.int) int {
	utils.Info.Printf("searchTree(): path=%s, anyDepth=%t, leafNodesOnly=%t", path, anyDepth, leafNodesOnly)
	if len(path) > 0 {
		// call int VSSSearchNodes(char* searchPath, long rootNode, int maxFound, searchData_t* searchData, bool anyDepth, bool leafNodesOnly, int* validation);
		cpath := C.CString(path)
		var matches C.int = C.VSSSearchNodes(cpath, rootNode, 150, (*C.struct_searchData_t)(unsafe.Pointer(searchData)), anyDepth, leafNodesOnly, (*C.int)(unsafe.Pointer(validation)))
		C.free(unsafe.Pointer(cpath))
		return int(matches)
	} else {
		return 0
	}
}

func getPathLen(path string) int {
	for i := 0; i < len(path); i++ {
		if path[i] == 0x00 { // the path buffer defined in searchData_t is initiated with all zeros
			return i
		}
	}
	return len(path)
}

func synthesizeValueObject(path string, value string) string {
	return `{"path":"` + path + `", "value":"` + value + `"}`
}

/**
* aggregateValue synthezises the "value" value when multiple matches may occur. The non-search response pattern for the value, "value": "123",
* is not sufficient as the response does not contain the corresponding path. So the following pattern is then used:
* For single match search result:
* {"path": "path-to-match", "value": "123"}
* For multiple match search result:
* "[{"path": "path-to-match1", "value": "123"}, {"path": "path-to-match2", "value": "456"}, ..]
**/
func aggregateValue(iterator int, path string, response string, aggregatedValue *string) {

	var responseMap map[string]interface{}
	utils.ExtractPayload(response, &responseMap)
	value := responseMap["value"].(string)

	switch responseMap["action"] {
	case "get":
		switch iterator {
		case 0:
			*aggregatedValue += synthesizeValueObject(path, value)
		case 1:
			*aggregatedValue = "[" + *aggregatedValue + ", " + synthesizeValueObject(path, value) + "]"
		default:
			*aggregatedValue = (*aggregatedValue)[:len(*aggregatedValue)-1] + ", " + synthesizeValueObject(path, value) + "]"
		}
	default: // set, subscribe: shall multiple matches be allowed??

	}
}

func setTokenErrorResponse(reqMap map[string]interface{}, errorCode int) {
	switch errorCode {
	case 1:
		utils.SetErrorResponse(reqMap, errorResponseMap, "400", "Token missing.", "")
	case 2:
		utils.SetErrorResponse(reqMap, errorResponseMap, "400", "Invalid token signature.", "")
	case 3:
		utils.SetErrorResponse(reqMap, errorResponseMap, "400", "Insufficient token permission.", "")
	case 4:
		utils.SetErrorResponse(reqMap, errorResponseMap, "400", "Token expired.", "")
	}
}

func verifyTokenSignature(token string) bool {
	hostIp := utils.GetServerIP()
	url := "http://" + hostIp + ":8600/atserver"
	utils.Info.Printf("verifyTokenSignature::url = %s", url)

	data := []byte(`{"token": "` + token + `"}`)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		utils.Error.Print("verifyTokenSignature: Error reading request. ", err)
		return false
	}

	// Set headers
	req.Header.Set("Access-Control-Allow-Origin", "*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", hostIp+":8600")

	// Set client timeout
	client := &http.Client{Timeout: time.Second * 10}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		utils.Error.Print("verifyTokenSignature: Error reading response. ", err)
		return false
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		utils.Error.Print("Error reading response. ", err)
		return false
	}

	if strings.Contains(string(body), "true") {
		return true
	}
	return false
}

func verifyToken(token string, validation int) int { // TODO verify expiry and other time stamps
	if verifyTokenSignature(token) == false {
		utils.Warning.Printf("verifyToken:invalid signature=%s", token)
		return 2
	}
	scope := utils.ExtractFromToken(token, "scp")
	if validation == 1 {
		if strings.Contains(scope, "Read") == false && strings.Contains(scope, "Control") == false {
			utils.Warning.Printf("verifyToken:Invalid scope=%s", token)
			return 3
		}
	} else {
		if strings.Contains(scope, "Control") == false {
			utils.Warning.Printf("verifyToken:Invalid scope=%s", token)
			return 3
		}
	}
	return 0
}

func isDataMatch(queryData string, response string) bool {
	var responsetMap = make(map[string]interface{})
	utils.ExtractPayload(response, &responsetMap)
	utils.Info.Printf("isDataMatch:queryData=%s, value=%s", queryData, responsetMap["value"].(string))
	if responsetMap["value"].(string) == queryData {
		return true
	}
	return false
}

func nextQuoteMark(message string) int {
    for i := 0 ; i < len(message) ; i++ {
        if (message[i] == '"') {
            return i
        }
    }
    return -1
}

func modifyResponse(resp string, aggregatedValue string) string {  // "value":"xxx" OR "value":"["xxx","yyy",..]"
    index := strings.Index(resp, "value") + 5
    quoteIndex1 := nextQuoteMark(resp[index+1:])
utils.Info.Printf("quoteIndex1=%d", quoteIndex1)
    quoteIndex2 := 0
    if (strings.Contains(resp[index+1:] , "\"[") == false) {
        quoteIndex2 = nextQuoteMark(resp[index+1+quoteIndex1+1:])
    } else {
        quoteIndex2 = strings.Index(resp[index+1+quoteIndex1+1:], "]\"") + 1
    }
utils.Info.Printf("quoteIndex2=%d", quoteIndex2)
    return resp[:index+1+quoteIndex1] + aggregatedValue + resp[index+1+quoteIndex1+1+quoteIndex2+1:]
}

func retrieveServiceResponse(requestMap map[string]interface{}, tDChanIndex int, sDChanIndex int, filterList []filterDef_t) {
	searchData := [150]searchData_t{} // vssparserutilities.h: #define MAXFOUNDNODES 150
	var anyDepth C.bool = false
	path := removeQuery(requestMap["path"].(string))
	if path[len(path)-1] == '*' {
		anyDepth = true
	}
	var validation C.int = -1
	matches := searchTree(VSSTreeRoot, path, &searchData[0], anyDepth, true, &validation)
	utils.Info.Printf("Max validation from search=%d", int(validation))
	if matches == 0 {
		utils.SetErrorResponse(requestMap, errorResponseMap, "400", "No signals matching path.", "")
		transportDataChan[tDChanIndex] <- utils.FinalizeMessage(errorResponseMap)
		return
	} else {
		switch int(validation) {
		case 0: // validation not required
		case 1:
			fallthrough
		case 2:
			errorCode := 0
			if requestMap["authorization"] == nil {
				errorCode = 1
			} else {
				if requestMap["action"] != "get" || int(validation) != 1 { // no validation for read requests when validation is 1 (write-only)
					errorCode = verifyToken(requestMap["authorization"].(string), int(validation))
				}
			}
			if errorCode > 0 {
				setTokenErrorResponse(requestMap, errorCode)
				transportDataChan[tDChanIndex] <- utils.FinalizeMessage(errorResponseMap)
				return
			}
		default: // should not be possible...
			utils.SetErrorResponse(requestMap, errorResponseMap, "400", "VSS access restriction tag invalid.", "See VSS2.0 spec for access restriction tagging")
			transportDataChan[tDChanIndex] <- utils.FinalizeMessage(errorResponseMap)
			return
		}
		var response string
		var aggregatedValue string
		var foundMatch int = 0
		var dataQuery bool = false
		var queryData string
		if listContainsName(filterList, "$data") == true {
			dataQuery = true
			queryData = getListValue(filterList, "$data")
		}
		for i := 0; i < matches; i++ {
			pathLen := getPathLen(string(searchData[i].responsePath[:]))
			requestMap["path"] = string(searchData[i].responsePath[:pathLen]) + addQuery(requestMap["path"].(string))

			serviceDataChan[sDChanIndex] <- utils.FinalizeMessage(requestMap)
			response = <-serviceDataChan[sDChanIndex]
			if dataQuery == false || (dataQuery == true && isDataMatch(queryData, response) == true) {
				if matches > 1 {
				    aggregateValue(foundMatch, requestMap["path"].(string), response, &aggregatedValue)
				}
				foundMatch++
			}

		}
		if foundMatch == 0 {
			utils.SetErrorResponse(requestMap, errorResponseMap, "400", "Data not matching query.", "")
			transportDataChan[tDChanIndex] <- utils.FinalizeMessage(errorResponseMap)
		} else {
			if matches == 1 {
				transportDataChan[tDChanIndex] <- response
			} else {
utils.Info.Printf("aggregatedValue=%s", aggregatedValue)
	                       response = modifyResponse(response, aggregatedValue)
utils.Info.Printf("aggregatedResponse=%s", response)
				transportDataChan[tDChanIndex] <- response
			}
		}
	}
}

func removeQuery(path string) string {
	pathEnd := strings.Index(path, "?")
	if pathEnd != -1 {
		return path[:pathEnd]
	}
	return path
}

func addQuery(path string) string {
	queryStart := strings.Index(path, "?")
	if queryStart != -1 {
		return path[queryStart:]
	}
	return ""
}

// vssparserutilities.h: nodeTypes_t; 0-9 -> the data types, 10-16 -> the node types. Should be separated in the C code declarations...
func nodeTypesToString(nodeType int) string {
	switch nodeType {
	case 0:
		return "int8"
	case 1:
		return "uint8"
	case 2:
		return "int16"
	case 3:
		return "uint16"
	case 4:
		return "int32"
	case 5:
		return "uint32"
	case 6:
		return "double"
	case 7:
		return "float"
	case 8:
		return "boolean"
	case 9:
		return "string"
	case 10:
		return "sensor"
	case 11:
		return "actuator"
	case 12:
		return "stream"
	case 13:
		return "attribute"
	case 14:
		return "branch"
	default:
		return ""
	}
}

func jsonifyTreeNode(nodeHandle C.long, jsonBuffer string, depth int, maxDepth int) string {
	if depth >= maxDepth {
		return jsonBuffer
	}
	depth++
	var newJsonBuffer string
	nodeName := C.GoString(C.getName(nodeHandle))
	newJsonBuffer += `"` + nodeName + `":{`
	nodeType := int(C.VSSgetType(nodeHandle))
	newJsonBuffer += `"type":` + `"` + nodeTypesToString(nodeType) + `",`
	nodeDescr := C.GoString(C.getDescr(nodeHandle))
	newJsonBuffer += `"description":` + `"` + nodeDescr + `",`
	nodeNumofChildren := int(C.getNumOfChildren(nodeHandle))
	switch nodeType {
	case 14: // branch
	case 12: // stream
	case 10: // sensor
		fallthrough
	case 11: // actuator
		fallthrough
	case 13: // attribute
		// TODO Look for other metadata, unit, enum, ...
		nodeDatatype := int(C.VSSgetDatatype(nodeHandle))
		newJsonBuffer += `"datatype:"` + `"` + nodeTypesToString(nodeDatatype) + `",`
	default: // 0-9 -> the data types, should not occur here (needs to be separated in C code declarations...)
		return ""

	}
	if depth < maxDepth {
		if nodeNumofChildren > 0 {
			newJsonBuffer += `"children":` + "{"
		}
		for i := 0; i < nodeNumofChildren; i++ {
			childNode := C.long(C.getChild(nodeHandle, C.int(i)))
			newJsonBuffer += jsonifyTreeNode(childNode, jsonBuffer, depth, maxDepth)
		}
		if nodeNumofChildren > 0 {
			if newJsonBuffer[len(newJsonBuffer)-1] == ',' && newJsonBuffer[len(newJsonBuffer)-2] != '}' {
				newJsonBuffer = newJsonBuffer[:len(newJsonBuffer)-1]
			}
			newJsonBuffer += "},"
		}
	}
	if newJsonBuffer[len(newJsonBuffer)-1] == ',' && newJsonBuffer[len(newJsonBuffer)-2] != '}' {
		newJsonBuffer = newJsonBuffer[:len(newJsonBuffer)-1]
	}
	newJsonBuffer += "},"
	depth--
	return jsonBuffer + newJsonBuffer
}

func countPathSegments(path string) int {
	return strings.Count(path, ".") + 1
}

func synthesizeJsonTree(path string, depth string) string {
	var jsonBuffer string
	searchData := [150]searchData_t{} // vssparserutilities.h: #define MAXFOUNDNODES 150
	matches := searchTree(VSSTreeRoot, path, &searchData[0], false, false, nil)
	if matches < countPathSegments(path) {
		return ""
	}
	subTreeRoot := C.long(searchData[matches-1].foundNodeHandle)
	utils.Info.Printf("synthesizeJsonTree:subTreeRoot-name=%s", C.GoString(C.getName(subTreeRoot)))
	var maxDepth int
	if depth == "0" {
		maxDepth = 100
	} else {
		maxDepth, _ = strconv.Atoi(depth)
	}
	jsonBuffer = jsonifyTreeNode(subTreeRoot, jsonBuffer, 0, maxDepth)
	return "{" + jsonBuffer + "}"
}

func processOneFilter(filter string, filterList *[]filterDef_t) string {
	filterDef := filterDef_t{}
	filterRemoved := false
	if strings.Contains(filter, "$spec") == true {
		filterDef.name = "$spec"
		filterRemoved = true
	} else if strings.Contains(filter, "$path") == true {
		filterDef.name = "$path"
		filterRemoved = true
	} else if strings.Contains(filter, "$data") == true {
		filterDef.name = "$data"
		filterRemoved = true
	}
	if filterRemoved == true {
		valueStart := strings.Index(filter, "EQ")
		if valueStart != -1 {
			filterDef.operator = "eq"
		} else {
			valueStart = strings.Index(filter, "GT")
			if valueStart != -1 {
				filterDef.operator = "gt"
			} else {
				valueStart = strings.Index(filter, "LT")
				if valueStart != -1 {
					filterDef.operator = "lt"
				}
			}
		}
		filterDef.value = filter[valueStart+2:]
		*filterList = append(*filterList, filterDef)
		utils.Info.Printf("processOneFilter():filter.name=%s, filter.operator=%s, filter.value=%s", filterDef.name, filterDef.operator, filterDef.value)
		return ""
	}
	return filter
}

/**
* Remove the filters $spec, $path, $data from the query component of the path, and add a list component for each removed filter.
* The logic behind this is that filters $interval, $range, $change are passed on to service mgr, while the removed ones are handled by the servercore.
**/
func processFilters(path string, filterList *[]filterDef_t) string {
	queryDelim := strings.Index(path, "?")
	query := path[queryDelim+1:]
	if queryDelim == -1 {
		return path
	}
	numOfFilters := strings.Count(query, "AND") + 1 // 0=>1, 1=> 2, 2=>3, 3=>4
	utils.Info.Printf("processFilters():#filter=%d", numOfFilters)
	var processedQuery string = ""
	filterStart := 0
	for i := 0; i < numOfFilters; i++ {
		filterEnd := strings.Index(query[filterStart:], "AND")
		if filterEnd == -1 {
			filterEnd = len(query)
		}
		filter := query[filterStart:filterEnd]
		if len(processedQuery) == 0 {
			processedQuery = processOneFilter(filter, filterList)
		} else {
			processedQuery += "AND" + processOneFilter(filter, filterList)
		}
		filterStart = filterEnd + 3 //len(AND)=3
	}
	if len(processedQuery) > 0 {
		processedQuery = "?" + processedQuery
	}
	utils.Info.Printf("processFilters():processed path=%s", path[0:queryDelim]+processedQuery)
	return path[0:queryDelim] + processedQuery
}

func listContainsName(filterList []filterDef_t, name string) bool {
	for i := 0; i < len(filterList); i++ {
		if filterList[i].name == name {
			return true
		}
	}
	return false
}

func getListValue(filterList []filterDef_t, name string) string {
	for i := 0; i < len(filterList); i++ {
		if filterList[i].name == name {
			return filterList[i].value
		}
	}
	return ""
}

func serveRequest(request string, tDChanIndex int, sDChanIndex int) {
	var requestMap = make(map[string]interface{})
	utils.ExtractPayload(request, &requestMap)
	filterList := []filterDef_t{}
	if _, ok := requestMap["path"]; ok {
		requestMap["path"] = processFilters(requestMap["path"].(string), &filterList)
	}
	switch requestMap["action"] {
	case "get":
		if listContainsName(filterList, "$spec") == true {
			requestMap["metadata"] = synthesizeJsonTree(removeQuery(requestMap["path"].(string)), getListValue(filterList, "$spec")) //TODO restrict tree to depth (handle error case)
			delete(requestMap, "path")
			requestMap["timestamp"] = 1234
			transportDataChan[tDChanIndex] <- utils.FinalizeMessage(requestMap)
		} else {
			if listContainsName(filterList, "$path") == true {
				requestMap["path"] = removeQuery(requestMap["path"].(string)) + "." + getListValue(filterList, "$path") //When/if VSS changes to slash delimiter, update here
			}
			retrieveServiceResponse(requestMap, tDChanIndex, sDChanIndex, filterList)
		}
	case "set":
		retrieveServiceResponse(requestMap, tDChanIndex, sDChanIndex, nil) // filters currently not used here
	case "subscribe":
		if listContainsName(filterList, "$path") == true {
			requestMap["path"] = removeQuery(requestMap["path"].(string)) + "." + getListValue(filterList, "$path") + addQuery(requestMap["path"].(string)) //When/if VSS changes to slash delimiter, update here
		}
		retrieveServiceResponse(requestMap, tDChanIndex, sDChanIndex, filterList)

	case "unsubscribe":
		utils.Info.Printf("unsubscribe:request=%s", request)
		serviceDataChan[sDChanIndex] <- request
		response := <-serviceDataChan[sDChanIndex]
		transportDataChan[tDChanIndex] <- response
	default:
		utils.Warning.Printf("serveRequest():not implemented/unknown action=%s\n", requestMap["action"])
		utils.SetErrorResponse(requestMap, errorResponseMap, "400", "unknown action", "See Gen2 spec for valid request actions.")
		transportDataChan[tDChanIndex] <- utils.FinalizeMessage(errorResponseMap)
	}
}

func updateTransportRoutingTable(mgrId int, portNum int) {
	utils.Info.Printf("Dummy updateTransportRoutingTable, mgrId=%d, portnum=%d", mgrId, portNum)
}

type PathList struct {
	LeafPaths []string
}
var pathList PathList

func sortPathList(listFname string) {
	data, err := ioutil.ReadFile(listFname)
	if err != nil {
		utils.Error.Printf("Error reading %s: %s\n", listFname, err)
		return
	}
	err = json.Unmarshal([]byte(data), &pathList)
	if err != nil {
		utils.Error.Printf("Error unmarshal json=%s\n", err)
		return
	}
	sort.Strings(pathList.LeafPaths)
	file, _ := json.Marshal(pathList)
	_ = ioutil.WriteFile(listFname, file, 0644)
}

func createPathListFile(listFname string) {
	// call int VSSGetLeafNodesList(long rootNode, char* leafNodeList);
	clistFname := C.CString(listFname)
	C.VSSGetLeafNodesList(VSSTreeRoot, clistFname)
	C.free(unsafe.Pointer(clistFname))
	sortPathList(listFname)
}

func main() {
	utils.InitLog("servercore-log.txt", "./logs")

	if !initVssFile() {
		utils.Error.Fatal(" Tree file not found")
		return
	}
	createPathListFile("../vsspathlist.json")  // save in server directory, where transport managers will expect it to be

	initTransportDataServers(transportDataChan, backendChan)
	utils.Info.Printf("main():initTransportDataServers() executed...")
	transportRegChan := make(chan int, 2*2)
	go initTransportRegisterServer(transportRegChan)
	utils.Info.Printf("main():initTransportRegisterServer() executed...")
	serviceRegChan := make(chan string, 2)
	serviceIndex := 0 // index assigned to registered services
	go initServiceRegisterServer(serviceRegChan, &serviceIndex, backendChan)
	utils.Info.Printf("main():starting loop for channel receptions...")
	for {
		select {
		case portNum := <-transportRegChan: // save port no + transport mgr Id in routing table
			mgrId := <-transportRegChan
			updateTransportRoutingTable(mgrId, portNum)
		case request := <-transportDataChan[0]: // request from transport0 (=HTTP), verify it, and route matches to servicemgr, or execute and respond if servicemgr not needed
			serveRequest(request, 0, 0)
		case request := <-transportDataChan[1]: // request from transport1 (=WS), verify it, and route matches to servicemgr, or execute and respond if servicemgr not needed
			serveRequest(request, 1, 0)
			//        case xxx := <- transportDataChan[2]:  // implement when there is a 3rd transport protocol mgr
		case portNo := <-serviceRegChan: // save service data portnum and root node in routing table
			rootNode := <-serviceRegChan
			updateServiceRouting(portNo, rootNode)
			//        case xxx := <- serviceDataChan[0]:    // for asynchronous routing, instead of the synchronous above. ToDo?
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
